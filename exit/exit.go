// Package exit is the cli-go ↔ errors-go adapter leaf (Law 8): it maps the
// cli-go control-flow sentinels — [cli.ErrHelp], [cli.ErrNoCommand], and the
// typed *[cli.ValidationError] / "unknown command|subcommand" usage errors —
// into the errors-go exit-code vocabulary. errors-go is the SOLE owner of
// process exit (§3.5): cli-go does not own a second 0/1/2 vocabulary, and this
// package never calls os.Exit — it only annotates an error with an exit code
// via [errors.WithExitCode], leaving the single funnel (`errs.Exit(run())` at
// main) to do the exiting.
//
// This is a THIRD leaf sub-package by design (Law 8): the cli-go core stays
// zero-dependency, errors-go is imported only here, and tools that want the
// exit-code mapping import this leaf — never the cores into each other.
//
//	func main() {
//		errs.Exit(run()) // the single funnel
//	}
//
//	func run() error {
//		root := buildRoot()
//		// borealis.Execute wiring is deferred (in-flight); today:
//		err := app.Run(context.Background(), os.Args)
//		return exit.Map(err) // ErrHelp → 0, usage → EX_USAGE, etc.
//	}
package exit

import (
	stderrors "errors"
	"strings"

	"github.com/pleme-io/cli-go"
	errs "github.com/pleme-io/errors-go"
)

// Map annotates a cli-go-produced error with the errors-go exit code it should
// terminate the process with, returning the annotated error for the single
// `errs.Exit` funnel at main. The mapping (outermost annotation wins, since it
// wraps with [errs.WithExitCode]):
//
//   - nil                                  → nil (errs.Exit → ExitOK / 0).
//   - [cli.ErrHelp]                        → ExitOK (0): printing help on
//     request is a success, not a failure.
//   - [cli.ErrNoCommand]                   → ExitUsage (64): the command was
//     invoked with no subcommand.
//   - a *[cli.ValidationError]             → ExitUsage (64): a flag value was
//     rejected — a usage error.
//   - an "unknown command"/"unknown
//     subcommand"/parse usage error        → ExitUsage (64).
//   - any other error                      → returned unchanged, so
//     [errs.ExitCodeOf]'s severity/temporary reduction applies (a plain error
//     still maps to ExitError / 1).
//
// Map is idempotent-safe: an error that already carries an explicit exit code
// (e.g. one built with errs.WithExitCode) is returned unchanged so the original
// mapping is preserved.
func Map(err error) error {
	if err == nil {
		return nil
	}

	// ErrHelp is a clean, requested early exit — code 0.
	if stderrors.Is(err, cli.ErrHelp) {
		return errs.Wrap(err, "help requested", errs.WithExitCode(errs.ExitOK))
	}

	// No subcommand given is a usage error.
	if stderrors.Is(err, cli.ErrNoCommand) {
		return errs.Wrap(err, "no command given",
			errs.WithSeverity(errs.SeverityError),
			errs.WithExitCode(errs.ExitUsage))
	}

	// A rejected flag value is a usage error; preserve the typed detail.
	var ve *cli.ValidationError
	if stderrors.As(err, &ve) {
		return errs.Wrap(err, "invalid flag value",
			errs.WithSeverity(errs.SeverityError),
			errs.WithExitCode(errs.ExitUsage),
			errs.WithPublic(ve.Error()))
	}

	// Unknown command / subcommand / parse failures are usage errors. cli-go
	// reports these as plain fmt errors with a stable "cli: …" prefix; match on
	// that shape (behaviour, not a sentinel, since the offending token varies).
	if isUsageMessage(err) {
		return errs.Wrap(err, "usage error",
			errs.WithSeverity(errs.SeverityError),
			errs.WithExitCode(errs.ExitUsage))
	}

	// Everything else: leave it to errors-go's severity/temporary reduction.
	return err
}

// isUsageMessage reports whether err's message is one of cli-go's usage-class
// errors (unknown command/subcommand, not-runnable, or a flag parse failure).
func isUsageMessage(err error) bool {
	msg := err.Error()
	for _, sub := range []string{
		"unknown command",
		"unknown subcommand",
		"is not runnable",
		"cli: parse ",
	} {
		if strings.Contains(msg, sub) {
			return true
		}
	}
	return false
}
