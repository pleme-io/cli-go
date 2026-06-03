package exit

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/pleme-io/cli-go"
	errs "github.com/pleme-io/errors-go"
)

func TestMap_Nil(t *testing.T) {
	if got := Map(nil); got != nil {
		t.Errorf("Map(nil) = %v, want nil", got)
	}
	if code := errs.ExitCodeOf(Map(nil)); code != errs.ExitOK {
		t.Errorf("ExitCodeOf(Map(nil)) = %d, want %d", code, errs.ExitOK)
	}
}

func TestMap_ErrHelp(t *testing.T) {
	got := Map(cli.ErrHelp)
	if code := errs.ExitCodeOf(got); code != errs.ExitOK {
		t.Errorf("ErrHelp → exit %d, want %d (clean exit)", code, errs.ExitOK)
	}
	// Still recognisable as ErrHelp through the wrap.
	if !stderrors.Is(got, cli.ErrHelp) {
		t.Errorf("mapped error lost ErrHelp identity")
	}
}

func TestMap_ErrNoCommand(t *testing.T) {
	got := Map(cli.ErrNoCommand)
	if code := errs.ExitCodeOf(got); code != errs.ExitUsage {
		t.Errorf("ErrNoCommand → exit %d, want %d (EX_USAGE)", code, errs.ExitUsage)
	}
	if !stderrors.Is(got, cli.ErrNoCommand) {
		t.Errorf("mapped error lost ErrNoCommand identity")
	}
}

func TestMap_ValidationError(t *testing.T) {
	ve := &cli.ValidationError{Flag: "port", Value: "x", Err: stderrors.New("must be an integer")}
	got := Map(ve)
	if code := errs.ExitCodeOf(got); code != errs.ExitUsage {
		t.Errorf("ValidationError → exit %d, want %d (EX_USAGE)", code, errs.ExitUsage)
	}
	// The typed detail survives as the public message.
	if pub := errs.PublicOf(got); pub == "" {
		t.Errorf("ValidationError should set a public message, got empty")
	}
	var asVE *cli.ValidationError
	if !stderrors.As(got, &asVE) {
		t.Errorf("mapped error lost ValidationError identity")
	}
}

func TestMap_UnknownCommand(t *testing.T) {
	err := fmt.Errorf("cli: unknown command %q", "bogus")
	if code := errs.ExitCodeOf(Map(err)); code != errs.ExitUsage {
		t.Errorf("unknown command → exit %d, want %d (EX_USAGE)", code, errs.ExitUsage)
	}
}

func TestMap_UnknownSubcommand(t *testing.T) {
	err := fmt.Errorf("cli: unknown subcommand %q for %q", "bogus", "tool auth")
	if code := errs.ExitCodeOf(Map(err)); code != errs.ExitUsage {
		t.Errorf("unknown subcommand → exit %d, want %d", code, errs.ExitUsage)
	}
}

func TestMap_ParseError(t *testing.T) {
	err := fmt.Errorf("cli: parse %q: %w", "tool list", stderrors.New("flag provided but not defined: -nope"))
	if code := errs.ExitCodeOf(Map(err)); code != errs.ExitUsage {
		t.Errorf("parse error → exit %d, want %d", code, errs.ExitUsage)
	}
}

// A plain (non-usage) error is left to errors-go's severity reduction → 1.
func TestMap_GenericError(t *testing.T) {
	err := stderrors.New("database is down")
	got := Map(err)
	if got != err {
		t.Errorf("generic error should pass through unchanged")
	}
	if code := errs.ExitCodeOf(got); code != errs.ExitError {
		t.Errorf("generic error → exit %d, want %d (ExitError)", code, errs.ExitError)
	}
}

// An error that already carries a temporary classification keeps its
// errors-go-reduced code (Map leaves it alone).
func TestMap_TemporaryPreserved(t *testing.T) {
	err := errs.New("upstream timeout", errs.WithTemporary(true))
	got := Map(err)
	if code := errs.ExitCodeOf(got); code != errs.ExitTempFail {
		t.Errorf("temporary error → exit %d, want %d (EX_TEMPFAIL)", code, errs.ExitTempFail)
	}
}
