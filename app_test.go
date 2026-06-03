package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

// helper: build an app writing usage into buf, with a "list" command that
// records whether it ran and a nested "auth" group.
func testApp(buf *bytes.Buffer, ran *string) *App {
	app := NewApp("clint",
		WithVersion("5.0.22"),
		WithDescription("Akeyless CLI"),
		WithOutput(buf),
	)
	app.Add(
		Command{
			Name:    "list",
			Summary: "List secrets",
			Flags: func(fs *flag.FlagSet) {
				fs.String("path", "/", "secrets path")
			},
			Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
				*ran = "list:" + fs.Lookup("path").Value.String()
				return nil
			},
		},
		Command{
			Name:    "auth",
			Summary: "Authentication commands",
			Sub: []Command{
				{
					Name:    "login",
					Summary: "Log in",
					Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
						*ran = "auth login"
						return nil
					},
				},
			},
		},
	)
	return app
}

func TestApp_Dispatch(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantRan string
		wantErr error  // sentinel to errors.Is against; nil means no error
		errSub  string // substring expected in a non-sentinel error
	}{
		{
			name:    "top-level command runs",
			argv:    []string{"clint", "list"},
			wantRan: "list:/",
		},
		{
			name:    "command flag parsed and passed through",
			argv:    []string{"clint", "list", "-path", "/prod"},
			wantRan: "list:/prod",
		},
		{
			name:    "nested subcommand dispatches one level deep",
			argv:    []string{"clint", "auth", "login"},
			wantRan: "auth login",
		},
		{
			name:    "no command yields ErrNoCommand",
			argv:    []string{"clint"},
			wantErr: ErrNoCommand,
		},
		{
			name:    "help yields ErrHelp",
			argv:    []string{"clint", "--help"},
			wantErr: ErrHelp,
		},
		{
			name:    "-h yields ErrHelp",
			argv:    []string{"clint", "-h"},
			wantErr: ErrHelp,
		},
		{
			name:   "unknown command is an error",
			argv:   []string{"clint", "bogus"},
			errSub: `unknown command "bogus"`,
		},
		{
			name:   "unknown subcommand is an error",
			argv:   []string{"clint", "auth", "bogus"},
			errSub: `unknown subcommand "bogus"`,
		},
		{
			name:    "pure group with no child shows usage, ErrNoCommand",
			argv:    []string{"clint", "auth"},
			wantErr: ErrNoCommand,
		},
		{
			name:    "command help yields ErrHelp",
			argv:    []string{"clint", "list", "--help"},
			wantErr: ErrHelp,
		},
		{
			name:    "nested group help yields ErrHelp",
			argv:    []string{"clint", "auth", "--help"},
			wantErr: ErrHelp,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			var ran string
			app := testApp(&buf, &ran)

			err := app.Run(context.Background(), tc.argv)

			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want errors.Is(%v)", err, tc.wantErr)
				}
			case tc.errSub != "":
				if err == nil || !strings.Contains(err.Error(), tc.errSub) {
					t.Fatalf("err = %v, want containing %q", err, tc.errSub)
				}
			default:
				if err != nil {
					t.Fatalf("unexpected err = %v", err)
				}
			}
			if tc.wantRan != "" && ran != tc.wantRan {
				t.Errorf("ran = %q, want %q", ran, tc.wantRan)
			}
			if tc.wantRan == "" && ran != "" && tc.wantErr == nil && tc.errSub == "" {
				t.Errorf("ran = %q, want command not to run", ran)
			}
		})
	}
}

func TestApp_Version(t *testing.T) {
	for _, arg := range []string{"--version", "-v", "version"} {
		var buf bytes.Buffer
		var ran string
		app := testApp(&buf, &ran)
		if err := app.Run(context.Background(), []string{"clint", arg}); err != nil {
			t.Fatalf("%s: unexpected err %v", arg, err)
		}
		if got := strings.TrimSpace(buf.String()); got != "clint 5.0.22" {
			t.Errorf("%s: version output = %q, want %q", arg, got, "clint 5.0.22")
		}
	}
}

func TestApp_VersionLine_NoVersion(t *testing.T) {
	app := NewApp("yocli")
	if got := app.versionLine(); got != "yocli" {
		t.Errorf("versionLine = %q, want %q", got, "yocli")
	}
}

func TestApp_TopUsageContent(t *testing.T) {
	var buf bytes.Buffer
	var ran string
	app := testApp(&buf, &ran)
	_ = app.Run(context.Background(), []string{"clint", "--help"})

	out := buf.String()
	for _, want := range []string{
		"clint 5.0.22 — Akeyless CLI", // version + description header
		"Usage:",
		"Commands:",
		"auth", // sorted listing includes both commands
		"list",
		"Authentication commands",
		"List secrets",
		`Run "clint <command> --help"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("top usage missing %q\n--- got ---\n%s", want, out)
		}
	}
	// auth sorts before list.
	if strings.Index(out, "\n  auth") > strings.Index(out, "\n  list") {
		t.Errorf("commands not name-sorted:\n%s", out)
	}
}

func TestApp_CommandUsageContent(t *testing.T) {
	var buf bytes.Buffer
	var ran string
	app := testApp(&buf, &ran)
	_ = app.Run(context.Background(), []string{"clint", "list", "--help"})

	out := buf.String()
	for _, want := range []string{
		"List secrets",         // summary
		"Usage:\n  clint list", // command path
		"[flags]",              // it has flags
		"Flags:",               // flag listing header
		"-path",                // the registered flag
	} {
		if !strings.Contains(out, want) {
			t.Errorf("command usage missing %q\n--- got ---\n%s", want, out)
		}
	}
}

func TestApp_GroupUsageListsSubcommands(t *testing.T) {
	var buf bytes.Buffer
	var ran string
	app := testApp(&buf, &ran)
	_ = app.Run(context.Background(), []string{"clint", "auth"})

	out := buf.String()
	for _, want := range []string{"Subcommands:", "login", "<subcommand>"} {
		if !strings.Contains(out, want) {
			t.Errorf("group usage missing %q\n--- got ---\n%s", want, out)
		}
	}
}

// A bad flag (unknown -nope) must surface as a parse error, not a panic.
func TestApp_ParseError(t *testing.T) {
	var buf bytes.Buffer
	var ran string
	app := testApp(&buf, &ran)
	err := app.Run(context.Background(), []string{"clint", "list", "-nope"})
	if err == nil || !strings.Contains(err.Error(), "parse") {
		t.Fatalf("err = %v, want parse error", err)
	}
	if ran != "" {
		t.Errorf("command ran despite parse error: %q", ran)
	}
}

// A parent command with its own Run handles args when no child matches.
func TestApp_GroupWithDefaultRun(t *testing.T) {
	var buf bytes.Buffer
	var got string
	app := NewApp("tool", WithOutput(&buf))
	app.Add(Command{
		Name:    "target",
		Summary: "Manage targets",
		Sub: []Command{
			{Name: "create", Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
				got = "create"
				return nil
			}},
		},
		Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
			got = "default:" + strings.Join(args, ",")
			return nil
		},
	})

	// Matching child wins.
	if err := app.Run(context.Background(), []string{"tool", "target", "create"}); err != nil {
		t.Fatal(err)
	}
	if got != "create" {
		t.Errorf("got = %q, want create", got)
	}

	// Non-child positional falls through to the parent's Run.
	got = ""
	if err := app.Run(context.Background(), []string{"tool", "target", "list-me"}); err != nil {
		t.Fatal(err)
	}
	if got != "default:list-me" {
		t.Errorf("got = %q, want default:list-me", got)
	}
}

// Run's own error from a command propagates verbatim.
func TestApp_RunErrorPropagates(t *testing.T) {
	sentinel := errors.New("boom")
	var buf bytes.Buffer
	app := NewApp("tool", WithOutput(&buf))
	app.Add(Command{Name: "go", Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
		return sentinel
	}})
	if err := app.Run(context.Background(), []string{"tool", "go"}); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
}
