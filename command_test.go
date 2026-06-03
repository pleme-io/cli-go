package cli

import (
	"bytes"
	"context"
	"flag"
	"strings"
	"testing"
)

// An alias dispatches the same as the canonical name and is listed in usage.
func TestCommand_Aliases(t *testing.T) {
	var ran string
	var buf bytes.Buffer
	app := NewApp("tool", WithOutput(&buf))
	app.Add(Command{
		Name:    "list",
		Aliases: []string{"ls", "l"},
		Summary: "List things",
		Run:     func(ctx context.Context, a []string, fs *flag.FlagSet) error { ran = "list"; return nil },
	})

	for _, tok := range []string{"list", "ls", "l"} {
		ran = ""
		if err := app.Run(context.Background(), []string{"tool", tok}); err != nil {
			t.Fatalf("%s: %v", tok, err)
		}
		if ran != "list" {
			t.Errorf("%s did not dispatch to list", tok)
		}
	}

	// Aliases listed in the command's own usage (derived, Law 4).
	buf.Reset()
	_ = app.Run(context.Background(), []string{"tool", "list", "--help"})
	out := buf.String()
	if !strings.Contains(out, "Aliases:") || !strings.Contains(out, "ls, l") {
		t.Errorf("usage should list aliases:\n%s", out)
	}
}

// A hidden command is dispatchable but omitted from the usage listing.
func TestCommand_Hidden(t *testing.T) {
	var ran bool
	var buf bytes.Buffer
	app := NewApp("tool", WithOutput(&buf))
	app.Add(
		Command{Name: "visible", Summary: "shown", Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil }},
		Command{Name: "secret", Hidden: true, Summary: "hidden", Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error { ran = true; return nil }},
	)

	// Still dispatchable.
	if err := app.Run(context.Background(), []string{"tool", "secret"}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Errorf("hidden command did not run")
	}

	// Omitted from top-level usage.
	buf.Reset()
	_ = app.Run(context.Background(), []string{"tool", "--help"})
	out := buf.String()
	if strings.Contains(out, "secret") {
		t.Errorf("hidden command should not appear in usage:\n%s", out)
	}
	if !strings.Contains(out, "visible") {
		t.Errorf("visible command should appear in usage:\n%s", out)
	}
}

// Categories group commands under headings in the usage listing.
func TestCommand_Categories(t *testing.T) {
	var buf bytes.Buffer
	app := NewApp("tool", WithOutput(&buf))
	app.Add(
		Command{Name: "login", Category: "Auth", Summary: "log in", Run: noRun},
		Command{Name: "logout", Category: "Auth", Summary: "log out", Run: noRun},
		Command{Name: "get", Category: "Secrets", Summary: "get a secret", Run: noRun},
	)
	_ = app.Run(context.Background(), []string{"tool", "--help"})
	out := buf.String()
	if !strings.Contains(out, "Auth:") || !strings.Contains(out, "Secrets:") {
		t.Errorf("usage should show category headings:\n%s", out)
	}
	// Auth sorts before Secrets; login/logout under Auth, get under Secrets.
	if strings.Index(out, "Auth:") > strings.Index(out, "Secrets:") {
		t.Errorf("categories not sorted:\n%s", out)
	}
}

// A deprecated command runs but is flagged in the listing and in its own usage.
func TestCommand_Deprecated(t *testing.T) {
	var ran bool
	var buf bytes.Buffer
	app := NewApp("tool", WithOutput(&buf))
	app.Add(Command{
		Name:       "old",
		Summary:    "old command",
		Deprecated: "use `new` instead",
		Run:        func(ctx context.Context, a []string, fs *flag.FlagSet) error { ran = true; return nil },
	})

	// Still runs.
	if err := app.Run(context.Background(), []string{"tool", "old"}); err != nil {
		t.Fatal(err)
	}
	if !ran {
		t.Errorf("deprecated command did not run")
	}

	// Flagged in listing.
	buf.Reset()
	_ = app.Run(context.Background(), []string{"tool", "--help"})
	if !strings.Contains(buf.String(), "(deprecated)") {
		t.Errorf("listing should flag deprecated command:\n%s", buf.String())
	}

	// Flagged in its own usage.
	buf.Reset()
	_ = app.Run(context.Background(), []string{"tool", "old", "--help"})
	out := buf.String()
	if !strings.Contains(out, "DEPRECATED") || !strings.Contains(out, "use `new` instead") {
		t.Errorf("command usage should show deprecation:\n%s", out)
	}
}

// Long prose and Examples are rendered in the command's usage.
func TestCommand_LongAndExamples(t *testing.T) {
	var buf bytes.Buffer
	app := NewApp("tool", WithOutput(&buf))
	app.Add(Command{
		Name:    "deploy",
		Summary: "deploy the app",
		Long:    "Deploys the application to the target environment.\nRespects the --env flag.",
		Examples: []string{
			"tool deploy --env prod",
			"tool deploy --env staging --dry-run",
		},
		Run: noRun,
	})
	_ = app.Run(context.Background(), []string{"tool", "deploy", "--help"})
	out := buf.String()
	for _, want := range []string{
		"Deploys the application", // Long prose
		"Respects the --env flag",
		"Examples:",
		"tool deploy --env prod",
		"tool deploy --env staging --dry-run",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("usage missing %q:\n%s", want, out)
		}
	}
}

func noRun(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil }
