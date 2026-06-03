package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Command is a single subcommand in an [App]'s command tree.
//
// A Command either runs (Run set) or groups child commands (Sub non-empty), or
// both — a parent command with its own Run acts as the default when no child is
// named. The router builds a fresh [flag.FlagSet] for each invocation, calls
// Flags to register flags onto it, parses the remaining argv, runs any
// registered validators, and finally calls Run with the parsed set.
//
// # Help is DERIVED, never hand-formatted (Law 4)
//
// The help *structure* — the command listing, the flag table, enum value sets,
// defaults, env names — is computed mechanically from this typed data and the
// registered [Flag] values; it is never assembled by hand. The only authored
// content is *prose* ([Summary], [Command.Long], [Command.Examples]), which is
// typed data on the Command, rendered (never formatted) — eventually through
// borealis (deferred while borealis.Execute/Render is in-flight; the structural
// derivation runs today on the stdlib renderer).
type Command struct {
	// Name is the token used to select this command on the command line. It
	// must be non-empty and is matched case-sensitively.
	Name string
	// Summary is the one-line description shown in the parent's usage listing.
	Summary string
	// Flags, if set, registers flags onto the per-invocation FlagSet. Register
	// validators here too via [RegisterValidator], or — preferred — bind typed
	// [Flag] handles via [Flag.Bind], which carries validation-as-data and
	// auto-documents OneOf/enum, defaults, and env names.
	Flags func(fs *flag.FlagSet)
	// Run executes the command. args holds the positional (non-flag) arguments
	// left after parsing; fs is the parsed, validated flag set. Run is called
	// only after parsing and validation succeed.
	Run func(ctx context.Context, args []string, fs *flag.FlagSet) error
	// Sub holds optional child commands, enabling one (or more) levels of
	// nesting. When set, the router dispatches the next argv token to a child;
	// if no child matches and Run is set, Run handles it, otherwise usage is
	// printed.
	Sub []Command

	// Aliases are alternate selection tokens for this command, matched the same
	// way as Name. They are listed in the command's usage so help stays the
	// single source of truth (Law 4: derived, not authored).
	Aliases []string
	// Hidden, when true, omits this command from usage listings while leaving it
	// fully dispatchable — the convention for internal/experimental commands.
	Hidden bool
	// Category groups this command under a heading in the parent's usage
	// listing. Commands sharing a Category are listed together; the empty
	// category sorts first under no heading.
	Category string
	// Deprecated, when non-empty, marks this command as deprecated and supplies
	// the guidance shown in usage (e.g. "use `auth login` instead"). The command
	// still runs; the notice is derived into help, never hand-formatted.
	Deprecated string

	// Long is authored multi-line prose describing the command in detail, shown
	// in the command's own usage below the summary. It is typed data — rendered,
	// never hand-formatted into the structural help.
	Long string
	// Examples are authored example invocations, shown verbatim in the command's
	// usage. Typed data, rendered as a block; the structure around them (the
	// "Examples:" heading, indentation) is derived.
	Examples []string
}

// matches reports whether tok selects this command by its Name or any Alias.
func (c Command) matches(tok string) bool {
	if c.Name == tok {
		return true
	}
	for _, a := range c.Aliases {
		if a == tok {
			return true
		}
	}
	return false
}

// AppOption configures an [App] at construction time (the functional-options
// pattern). See [WithVersion] and [WithDescription].
type AppOption func(*App)

// WithVersion sets the App's version string, surfaced by the built-in
// "version" command and the --version flag.
func WithVersion(v string) AppOption { return func(a *App) { a.version = v } }

// WithDescription sets the App's one-line description, shown in top-level usage.
func WithDescription(d string) AppOption { return func(a *App) { a.description = d } }

// WithOutput redirects the App's usage and version output. The default is
// os.Stderr, matching the flag package and standard CLI convention.
func WithOutput(w io.Writer) AppOption { return func(a *App) { a.out = w } }

// App is a CLI application: a named root with a version, a description, and a
// registered set of top-level [Command]s. Construct one with [NewApp] and drive
// it with [App.Run].
type App struct {
	name        string
	version     string
	description string
	out         io.Writer
	commands    []Command
}

// NewApp creates an App with the given program name and options.
func NewApp(name string, opts ...AppOption) *App {
	a := &App{name: name, out: os.Stderr}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Add registers one or more top-level commands. It returns the App for
// chaining. Commands are dispatched in registration order; later registrations
// of the same Name shadow earlier ones.
func (a *App) Add(cmds ...Command) *App {
	a.commands = append(a.commands, cmds...)
	return a
}

// Name returns the program name.
func (a *App) Name() string { return a.name }

// Version returns the configured version string (empty if unset).
func (a *App) Version() string { return a.version }

// Run parses argv and dispatches to the selected command. argv is the full
// process argument vector including the program name at index 0 (pass
// os.Args directly).
//
// Behaviour:
//   - argv with no command (len <= 1) prints top-level usage and returns
//     [ErrNoCommand];
//   - "--help"/"-h"/"help" prints top-level usage and returns [ErrHelp];
//   - "--version"/"version" prints the version and returns nil;
//   - an unknown command prints top-level usage and returns an "unknown
//     command" error;
//   - a known command parses its flags, runs validators, and executes; a
//     command's own "--help"/"-h" prints that command's usage and returns
//     [ErrHelp].
func (a *App) Run(ctx context.Context, argv []string) error {
	args := argv
	if len(args) > 0 {
		args = args[1:] // drop program name
	}
	if len(args) == 0 {
		a.printUsage()
		return ErrNoCommand
	}

	switch args[0] {
	case "-h", "--help", "help":
		a.printUsage()
		return ErrHelp
	case "-v", "--version", "version":
		fmt.Fprintln(a.out, a.versionLine())
		return nil
	}

	cmd, ok := a.lookup(args[0])
	if !ok {
		a.printUsage()
		return fmt.Errorf("cli: unknown command %q", args[0])
	}
	return a.dispatch(ctx, cmd, a.name+" "+cmd.Name, args[1:])
}

// lookup finds a top-level command by name or alias (last registration wins).
func (a *App) lookup(name string) (Command, bool) {
	for i := len(a.commands) - 1; i >= 0; i-- {
		if a.commands[i].matches(name) {
			return a.commands[i], true
		}
	}
	return Command{}, false
}

// dispatch executes a (possibly nested) command. path is the human-readable
// command path used in usage (e.g. "clint auth"). args are the arguments after
// the command's own name.
func (a *App) dispatch(ctx context.Context, cmd Command, path string, args []string) error {
	// Nested dispatch: if the command has children and the next token names
	// one, recurse. A leading help token prints this command's usage.
	if len(cmd.Sub) > 0 {
		if len(args) > 0 {
			switch args[0] {
			case "-h", "--help", "help":
				a.printCommandUsage(cmd, path)
				return ErrHelp
			}
			if child, ok := findSub(cmd.Sub, args[0]); ok {
				return a.dispatch(ctx, child, path+" "+child.Name, args[1:])
			}
			// Unknown child with no own Run is an error; with a Run, fall
			// through and let this command handle the args itself.
			if cmd.Run == nil {
				a.printCommandUsage(cmd, path)
				return fmt.Errorf("cli: unknown subcommand %q for %q", args[0], path)
			}
		} else if cmd.Run == nil {
			// A pure group invoked with no child: show its usage.
			a.printCommandUsage(cmd, path)
			return ErrNoCommand
		}
	}

	if cmd.Run == nil {
		a.printCommandUsage(cmd, path)
		return fmt.Errorf("cli: command %q is not runnable", path)
	}

	fs := flag.NewFlagSet(path, flag.ContinueOnError)
	fs.SetOutput(a.out)
	reg := newRegistry(fs)
	fs.Usage = func() { a.writeCommandUsage(cmd, path, fs) }
	if cmd.Flags != nil {
		withRegistry(fs, reg, func() { cmd.Flags(fs) })
	}

	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return ErrHelp
		}
		return fmt.Errorf("cli: parse %q: %w", path, err)
	}
	if err := reg.validate(); err != nil {
		return err
	}
	return cmd.Run(ctx, fs.Args(), fs)
}

// findSub finds a child command by name or alias (last wins).
func findSub(subs []Command, name string) (Command, bool) {
	for i := len(subs) - 1; i >= 0; i-- {
		if subs[i].matches(name) {
			return subs[i], true
		}
	}
	return Command{}, false
}

// versionLine renders "name version" (or just "name" when no version is set).
func (a *App) versionLine() string {
	if a.version == "" {
		return a.name
	}
	return a.name + " " + a.version
}

// printUsage writes top-level usage to the App's output.
func (a *App) printUsage() {
	var b strings.Builder
	if a.description != "" {
		fmt.Fprintf(&b, "%s — %s\n\n", a.versionLine(), a.description)
	} else {
		fmt.Fprintf(&b, "%s\n\n", a.versionLine())
	}
	fmt.Fprintf(&b, "Usage:\n  %s <command> [flags]\n\n", a.name)
	fmt.Fprintf(&b, "Commands:\n")
	writeCommandTable(&b, a.commands)
	fmt.Fprintf(&b, "\nRun \"%s <command> --help\" for more information about a command.\n", a.name)
	fmt.Fprint(a.out, b.String())
}

// printCommandUsage writes a command's usage to the App's output. A throwaway
// FlagSet is constructed so the flag listing matches what Parse would accept.
func (a *App) printCommandUsage(cmd Command, path string) {
	fs := flag.NewFlagSet(path, flag.ContinueOnError)
	fs.SetOutput(a.out)
	if cmd.Flags != nil {
		reg := newRegistry(fs)
		withRegistry(fs, reg, func() { cmd.Flags(fs) })
	}
	a.writeCommandUsage(cmd, path, fs)
}

// writeCommandUsage renders the usage block for a command. Every section is
// DERIVED from the typed Command data (Law 4): the summary/long/examples prose
// is rendered, the deprecation notice and alias list are derived from the typed
// fields, the subcommand table is derived from Sub (Hidden filtered, Category
// grouped), and the flag table is derived from the FlagSet plus any registered
// typed [Flag] handles (OneOf/default/env).
func (a *App) writeCommandUsage(cmd Command, path string, fs *flag.FlagSet) {
	var b strings.Builder
	if cmd.Summary != "" {
		fmt.Fprintf(&b, "%s\n\n", cmd.Summary)
	}
	if cmd.Deprecated != "" {
		fmt.Fprintf(&b, "DEPRECATED: %s\n\n", cmd.Deprecated)
	}
	if cmd.Long != "" {
		fmt.Fprintf(&b, "%s\n\n", strings.TrimRight(cmd.Long, "\n"))
	}
	usage := "Usage:\n  " + path
	if len(cmd.Sub) > 0 {
		usage += " <subcommand>"
	}
	if hasFlags(fs) {
		usage += " [flags]"
	}
	fmt.Fprintf(&b, "%s\n", usage)
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(&b, "\nAliases:\n  %s\n", strings.Join(cmd.Aliases, ", "))
	}
	if len(cmd.Sub) > 0 {
		fmt.Fprintf(&b, "\nSubcommands:\n")
		writeCommandTable(&b, cmd.Sub)
	}
	fmt.Fprint(a.out, b.String())
	if hasFlags(fs) {
		fmt.Fprintf(a.out, "\nFlags:\n")
		writeFlagTable(a.out, fs)
	}
	if len(cmd.Examples) > 0 {
		var e strings.Builder
		fmt.Fprintf(&e, "\nExamples:\n")
		for _, ex := range cmd.Examples {
			fmt.Fprintf(&e, "  %s\n", ex)
		}
		fmt.Fprint(a.out, e.String())
	}
}

// hasFlags reports whether the FlagSet has any registered flags.
func hasFlags(fs *flag.FlagSet) bool {
	n := 0
	fs.VisitAll(func(*flag.Flag) { n++ })
	return n > 0
}

// writeFlagTable renders the flag listing. The structure (names, types,
// defaults) is DERIVED by the stdlib flag package's PrintDefaults; the typed
// [Flag] handles fold their OneOf set and env name into each flag's usage
// string at Bind time, so the enum/env documentation is derived from the typed
// Flag data rather than hand-formatted (Law 4). The borealis-styled rendering
// of this table is deferred to borealis.Execute (in-flight). fs is expected to
// already have its output set to w by the caller (dispatch/printCommandUsage).
func writeFlagTable(w io.Writer, fs *flag.FlagSet) {
	fs.SetOutput(w)
	fs.PrintDefaults()
}

// writeCommandTable writes an aligned "  name  summary" table, derived from the
// typed Command data: Hidden commands are filtered out, the remainder is grouped
// by Category (empty category first, under no heading), and within each group
// commands are name-sorted. A deprecated command is flagged inline. The table is
// computed mechanically — it is never hand-formatted (Law 4).
func writeCommandTable(b *strings.Builder, cmds []Command) {
	visible := make([]Command, 0, len(cmds))
	for _, c := range cmds {
		if !c.Hidden {
			visible = append(visible, c)
		}
	}
	if len(visible) == 0 {
		return
	}

	// Stable category order: first appearance, with "" (uncategorized) first.
	var catOrder []string
	byCat := map[string][]Command{}
	for _, c := range visible {
		if _, seen := byCat[c.Category]; !seen {
			catOrder = append(catOrder, c.Category)
		}
		byCat[c.Category] = append(byCat[c.Category], c)
	}
	sort.SliceStable(catOrder, func(i, j int) bool {
		if catOrder[i] == "" {
			return true
		}
		if catOrder[j] == "" {
			return false
		}
		return catOrder[i] < catOrder[j]
	})

	// Width is computed over the whole visible set so columns align across
	// categories.
	width := 0
	for _, c := range visible {
		if len(c.Name) > width {
			width = len(c.Name)
		}
	}

	multiCat := len(catOrder) > 1 || (len(catOrder) == 1 && catOrder[0] != "")
	for _, cat := range catOrder {
		group := byCat[cat]
		sort.Slice(group, func(i, j int) bool { return group[i].Name < group[j].Name })
		if multiCat && cat != "" {
			fmt.Fprintf(b, "\n %s:\n", cat)
		}
		for _, c := range group {
			summary := c.Summary
			if c.Deprecated != "" {
				summary = strings.TrimSpace(summary + " (deprecated)")
			}
			fmt.Fprintf(b, "  %-*s  %s\n", width, c.Name, summary)
		}
	}
}
