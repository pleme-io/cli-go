// Package cli is the Go representation of pleme-io's CLI framework — the Go
// counterpart to the Rust `clap` / `caixa-clap` model. It gives Go tools the
// same shape everywhere: a named App with a version and description, a tree of
// subcommands, per-flag validators run after parsing, and a small multi-auth
// resolver — all on the standard library's [flag] package, with zero external
// dependencies.
//
// The mandate, like the Rust crates: no hand-rolled argv switch statements, no
// bespoke flag parsing per tool. Build an App, Add commands, hand it argv, and
// every binary in the fleet dispatches, validates, and prints usage the same
// way.
//
//	app := cli.NewApp("clint",
//		cli.WithVersion("5.0.22"),
//		cli.WithDescription("Akeyless CLI"),
//	)
//	app.Add(cli.Command{
//		Name:    "list-secrets",
//		Summary: "List secrets under a path",
//		Flags: func(fs *flag.FlagSet) {
//			fs.String("path", "/", "secrets path")
//		},
//		Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
//			// fs is parsed and validated here.
//			return nil
//		},
//	})
//	if err := app.Run(context.Background(), os.Args); err != nil {
//		log.Fatal(err)
//	}
//
// Subcommands may nest one level via [Command.Sub]; the router dispatches on the
// first non-flag token and prints usage for an unknown command or --help/-h.
//
// # Borealis elevation (§2.2)
//
// This package is the fleet CLI *authoring* surface; under the Borealis Theory
// it carries the typed grammar (single source of truth) while presentation is
// owned by borealis. The elevated surface adds, additively over the original:
//
//   - Typed validation-as-data: [Flag] is a generic, self-describing flag whose
//     name/default/env/OneOf/Validate travel together, so help STRUCTURE
//     (types, enum sets, defaults, env names) is DERIVED from the typed data,
//     never hand-formatted (Law 4). Read a parsed, validated, typed value with
//     [Flag.Get].
//   - The fleet-standard persistent flag set ([FleetFlags]): --auth (its value
//     set auto-wired from [AuthResolver.Kinds]), --profile, --gateway-url,
//     --output, --no-color (NO_COLOR), --verbose — defined once, inherited
//     everywhere.
//   - A raised [Command] type: Aliases, Hidden, Category, Deprecated (derived
//     into help) plus authored prose (Long, Examples — typed data, rendered).
//
// The dep-bearing integrations live in leaf sub-packages so the core stays
// zero-dependency (Law 6/8): cli-go/cfg wires the canonical shikumi loader to a
// primitive's FromConfig (§3.5), and cli-go/exit maps the cli sentinels into
// errors-go exit codes (errors-go is the sole exit owner, §3.5). The
// borealis.Execute entrypoint and borealis-rendered styled help/errors are
// DEFERRED while borealis.Execute/Render is in-flight — the root [Command]
// defined here is what borealis.Execute will consume; this package does not (and
// must not) redeclare Execute.
package cli

import "errors"

// ErrHelp is returned by [App.Run] (and command routing) when help output was
// requested via --help/-h and printed. Callers typically treat it as a clean,
// non-error exit:
//
//	if err := app.Run(ctx, os.Args); err != nil && !errors.Is(err, cli.ErrHelp) {
//		log.Fatal(err)
//	}
var ErrHelp = errors.New("cli: help requested")

// ErrNoCommand is returned by [App.Run] when argv carries no subcommand at all
// (only the program name). The router prints top-level usage before returning
// it.
var ErrNoCommand = errors.New("cli: no command given")
