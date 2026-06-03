package cobrax_test

import (
	"bytes"
	"context"
	stdflag "flag"
	"strings"
	"testing"

	cli "github.com/pleme-io/cli-go"
	"github.com/pleme-io/cli-go/cobrax"
)

// Build lowers the typed tree and cobra dispatches a subcommand into the cli-go
// engine, with flags parsed + the typed Run reached.
func TestBuild_DispatchesSubcommand(t *testing.T) {
	var ran string
	var gotMode string
	mode := cli.NewFlag[string]("mode", "fast", "execution mode").OneOf("fast", "slow")

	app := cli.NewApp("tool", cli.WithVersion("1.2.3"), cli.WithDescription("a tool"))
	app.Add(cli.Command{
		Name:    "run",
		Summary: "run the thing",
		Flags:   func(fs *stdflag.FlagSet) { mode.Bind(fs) },
		Run: func(ctx context.Context, args []string, fs *stdflag.FlagSet) error {
			ran = "run"
			gotMode = mode.Get()
			return nil
		},
	})

	root := cobrax.Build(app)
	root.SetArgs([]string{"run", "-mode", "slow"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if ran != "run" {
		t.Errorf("subcommand did not dispatch into the cli-go engine")
	}
	if gotMode != "slow" {
		t.Errorf("typed Flag.Get() = %q, want slow (parsed by the cli-go engine)", gotMode)
	}
}

// A nested subcommand dispatches through the lowered tree.
func TestBuild_NestedDispatch(t *testing.T) {
	var ran string
	app := cli.NewApp("tool")
	app.Add(cli.Command{
		Name:    "auth",
		Summary: "auth ops",
		Sub: []cli.Command{{
			Name:    "login",
			Summary: "log in",
			Run: func(ctx context.Context, args []string, fs *stdflag.FlagSet) error {
				ran = "login"
				return nil
			},
		}},
	})

	root := cobrax.Build(app)
	root.SetArgs([]string{"auth", "login"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext: %v", err)
	}
	if ran != "login" {
		t.Errorf("nested command did not dispatch, ran=%q", ran)
	}
}

// The typed-Flag validation path is preserved: a bad OneOf value is rejected by
// the shared cli-go engine, surfacing the typed *ValidationError.
func TestBuild_ValidationFlowsThrough(t *testing.T) {
	mode := cli.NewFlag[string]("mode", "fast", "mode").OneOf("fast", "slow")
	app := cli.NewApp("tool")
	app.Add(cli.Command{
		Name:  "run",
		Flags: func(fs *stdflag.FlagSet) { mode.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *stdflag.FlagSet) error { return nil },
	})

	root := cobrax.Build(app)
	root.SetArgs([]string{"run", "-mode", "weird"})
	err := root.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected a validation error for a bad OneOf value")
	}
	if !strings.Contains(err.Error(), "mode") {
		t.Errorf("error %q should name the offending flag", err.Error())
	}
}

// Help renders: cobra (which fang styles) shows the command's summary, derived
// flag table (with the auto-documented OneOf/env), and authored prose.
func TestBuild_RendersHelp(t *testing.T) {
	mode := cli.NewFlag[string]("mode", "fast", "execution mode").
		Env("TOOL_MODE").OneOf("fast", "slow")
	app := cli.NewApp("tool", cli.WithDescription("a tool"))
	app.Add(cli.Command{
		Name:     "run",
		Summary:  "run the thing",
		Long:     "Runs the thing in the chosen mode.",
		Examples: []string{"tool run -mode slow"},
		Flags:    func(fs *stdflag.FlagSet) { mode.Bind(fs) },
		Run:      func(ctx context.Context, a []string, fs *stdflag.FlagSet) error { return nil },
	})

	root := cobrax.Build(app)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"run", "--help"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("help: %v", err)
	}
	// cobra renders a leaf command's body from Long (the authored prose); the
	// one-line Summary surfaces in the parent's command listing (asserted by
	// TestBuild_RootHelpListsCommands). Here we assert the body + derived table.
	out := buf.String()
	for _, want := range []string{
		"Runs the thing",      // authored Long prose
		"mode",                // derived flag
		"execution mode",      // flag usage
		"one of: fast, slow",  // derived OneOf set (Law 4)
		"[env: TOOL_MODE]",    // derived env name (Law 4)
		"tool run -mode slow", // authored Example
	} {
		if !strings.Contains(out, want) {
			t.Errorf("help missing %q:\n%s", want, out)
		}
	}
}

// The top-level help lists the registered commands and the description.
func TestBuild_RootHelpListsCommands(t *testing.T) {
	app := cli.NewApp("tool", cli.WithDescription("a tool"))
	app.Add(
		cli.Command{Name: "alpha", Summary: "first", Run: noRun},
		cli.Command{Name: "beta", Summary: "second", Run: noRun},
	)
	root := cobrax.Build(app)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"--help"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("root help: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"a tool", "alpha", "first", "beta", "second"} {
		if !strings.Contains(out, want) {
			t.Errorf("root help missing %q:\n%s", want, out)
		}
	}
}

func noRun(ctx context.Context, a []string, fs *stdflag.FlagSet) error { return nil }
