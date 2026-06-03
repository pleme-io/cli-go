// Package cobrax is the GATED LEAF that lowers cli-go's typed grammar tree
// ([cli.App] / [cli.Command] / [cli.Flag]) onto a [*cobra.Command] (Law 8: the
// cli-go CORE stays pure-stdlib zero-dependency; cobra is imported only here).
//
// # Why this leaf exists (BOREALIS §2.2 / §2.9)
//
// The Borealis Theory keeps grammar and presentation separate: cli-go owns the
// *grammar* (the single source of truth — one typed tree that derives parse,
// validation, and help structure), and borealis owns 100% of *presentation* via
// fang. fang is specifically a cobra decorator, so adopting borealis.Execute
// makes cobra the parser-of-record fleet-wide (§2.2). This leaf is the bridge:
// it renders the typed cli-go tree as a cobra command so borealis.Execute can
// hand it to fang.
//
// # The bridge contract: grammar lowered, engine reused (PRIME DIRECTIVE)
//
// cobra owns ONLY the command tree, the help/usage shell, --version, man pages,
// and shell completions (everything fang styles). The actual flag parse +
// typed-[cli.Flag] validation + Run dispatch stays in cli-go's existing engine
// ([cli.App.ExecCommand]) — this leaf does NOT re-implement parsing or
// validation (duplication is a bug). Each lowered leaf command runs with
// cobra's flag parsing disabled and delegates its raw args straight to
// [cli.App.ExecCommand], so a tool behaves byte-identically whether it is run
// through the stdlib router ([cli.App.Run]) or through cobra+fang
// (borealis.Execute). The flag *table* shown in help is DERIVED by replaying the
// command's Flags callback onto a throwaway stdlib FlagSet and adapting it onto
// cobra's pflag set (Law 4: help structure is derived, never hand-formatted).
//
//	root := cli.NewApp("clint", cli.WithVersion("5.0.22"),
//		cli.WithDescription("Akeyless CLI"))
//	root.Add(cli.Command{Name: "list", Summary: "list secrets", Run: run})
//	return fangx.Execute(ctx, cobrax.Build(root), borealis.Nord())
//	// or, fleet-uniform: return borealis.Execute(ctx, root)
package cobrax

import (
	stdflag "flag"
	"strings"

	"github.com/spf13/cobra"

	cli "github.com/pleme-io/cli-go"
)

// Build lowers a cli-go [cli.App] onto a root [*cobra.Command], recursively
// lowering its command tree. The root carries the App's name (Use), version,
// and description (Short); cobra's built-in completion + help commands are left
// enabled so fang can style them. The returned command is ready to hand to
// [github.com/pleme-io/borealis/fangx.Execute] (or borealis.Execute, which calls
// it for you).
//
// ctx is threaded into every lowered command's RunE so the typed cli-go Run
// receives the same context borealis.Execute was given (cobra's
// ExecuteContext already does this, but Build is context-free at construction;
// the context arrives at run time via cobra).
func Build(app *cli.App) *cobra.Command {
	root := &cobra.Command{
		Use:           app.Name(),
		Short:         app.Description(),
		Version:       app.Version(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if w := app.Output(); w != nil {
		root.SetOut(w)
		root.SetErr(w)
	}
	for _, c := range app.Commands() {
		root.AddCommand(lower(app, c, app.Name()))
	}
	return root
}

// lower turns one cli-go [cli.Command] into a [*cobra.Command], recursing over
// Sub. path is the human-readable command path (e.g. "clint auth") threaded into
// [cli.App.ExecCommand] so parse/usage errors carry the full path, matching the
// stdlib router.
func lower(app *cli.App, c cli.Command, parentPath string) *cobra.Command {
	path := parentPath + " " + c.Name
	cc := &cobra.Command{
		Use:           c.Name,
		Short:         c.Summary,
		Long:          longText(c),
		Aliases:       c.Aliases,
		Hidden:        c.Hidden,
		Example:       strings.Join(c.Examples, "\n"),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if c.Deprecated != "" {
		cc.Deprecated = c.Deprecated
	}
	// Derive the flag table for help (Law 4): replay the typed Flags callback
	// onto a throwaway stdlib FlagSet, then adapt it onto cobra's pflag set so
	// fang renders the same flags the engine will parse. Parsing itself stays in
	// cli-go's engine, so we disable cobra's parsing and forward raw args.
	addDerivedFlags(cc, c)

	// A runnable command delegates to the shared cli-go engine. A pure group
	// (no Run) has no RunE — cobra prints its subcommand help, which fang styles.
	if c.Run != nil {
		cmd := c // capture for the closure
		cc.DisableFlagParsing = true
		cc.RunE = func(cobraCmd *cobra.Command, args []string) error {
			// A help request on a runnable command: let cobra render styled help.
			if wantsHelp(args) {
				return cobraCmd.Help()
			}
			return app.ExecCommand(cobraCmd.Context(), cmd, path, args)
		}
	}

	for _, sub := range c.Sub {
		cc.AddCommand(lower(app, sub, path))
	}
	return cc
}

// addDerivedFlags replays the command's Flags callback against a throwaway
// stdlib FlagSet and copies the resulting flags onto cobra's pflag set purely so
// the help table is DERIVED from the same typed data the engine parses (Law 4).
// pflag.AddGoFlagSet is the canonical stdlib→pflag adapter. Because the lowered
// command sets DisableFlagParsing, these pflag entries are display-only — the
// engine ([cli.App.ExecCommand]) re-binds and parses them itself.
func addDerivedFlags(cc *cobra.Command, c cli.Command) {
	if c.Flags == nil {
		return
	}
	fs := stdflag.NewFlagSet(c.Name, stdflag.ContinueOnError)
	// The typed cli.Flag handles auto-document OneOf/env into each flag's usage
	// string at Bind time, so adapting the stdlib FlagSet carries that derived
	// help across unchanged.
	c.Flags(fs)
	cc.Flags().AddGoFlagSet(fs)
}

// longText is the long-form help body from the command's authored prose (Long).
// It is typed data, rendered, never hand-formatted into the structural help; the
// deprecation notice is carried separately via cobra's own Deprecated field.
func longText(c cli.Command) string { return strings.TrimRight(c.Long, "\n") }

// wantsHelp reports whether the raw args (DisableFlagParsing is on, so cobra
// hands them through verbatim) request help. The stdlib engine also recognises
// --help/-h and returns cli.ErrHelp, but routing the help render through cobra
// lets fang style it uniformly.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			return true
		}
		if a == "--" {
			break // everything after -- is positional
		}
	}
	return false
}
