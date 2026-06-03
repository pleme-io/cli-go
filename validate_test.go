package cli

import (
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

// A passing validator lets the command run; a failing one stops it with a
// typed *ValidationError.
func TestValidator_PassAndFail(t *testing.T) {
	tests := []struct {
		name    string
		argv    []string
		wantRun bool
		wantBad string // expected ValidationError.Flag, "" if no error
	}{
		{
			name:    "valid port runs",
			argv:    []string{"app", "serve", "-port", "8080"},
			wantRun: true,
		},
		{
			name:    "port below range fails",
			argv:    []string{"app", "serve", "-port", "0"},
			wantBad: "port",
		},
		{
			name:    "port above range fails",
			argv:    []string{"app", "serve", "-port", "70000"},
			wantBad: "port",
		},
		{
			name:    "missing required env fails",
			argv:    []string{"app", "serve", "-port", "9090", "-env", ""},
			wantBad: "env",
		},
		{
			name:    "bad enum fails",
			argv:    []string{"app", "serve", "-port", "9090", "-env", "x", "-mode", "weird"},
			wantBad: "mode",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ran bool
			app := NewApp("app")
			app.Add(Command{
				Name: "serve",
				Flags: func(fs *flag.FlagSet) {
					fs.Int("port", 8080, "listen port")
					fs.String("env", "prod", "environment")
					fs.String("mode", "fast", "mode")
					RegisterValidator(fs, "port", Range(1, 65535))
					RegisterValidator(fs, "env", Required())
					RegisterValidator(fs, "mode", OneOf("fast", "slow"))
				},
				Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
					ran = true
					return nil
				},
			})

			err := app.Run(context.Background(), tc.argv)

			if tc.wantBad == "" {
				if err != nil {
					t.Fatalf("unexpected err %v", err)
				}
				if !ran {
					t.Errorf("command did not run")
				}
				return
			}

			if ran {
				t.Errorf("command ran despite validation failure")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("err = %v, want *ValidationError", err)
			}
			if ve.Flag != tc.wantBad {
				t.Errorf("ValidationError.Flag = %q, want %q", ve.Flag, tc.wantBad)
			}
			if !strings.Contains(ve.Error(), "-"+tc.wantBad) {
				t.Errorf("error message %q does not mention flag", ve.Error())
			}
		})
	}
}

// Range on a string flag doubles as an int-shape check: a typed int flag would
// reject non-integers at Parse, but on a string flag the validator catches it.
func TestValidator_RangeOnStringFlag(t *testing.T) {
	app := NewApp("app")
	app.Add(Command{
		Name: "go",
		Flags: func(fs *flag.FlagSet) {
			fs.String("n", "1", "a count, as a string")
			RegisterValidator(fs, "n", Range(1, 10))
		},
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})
	err := app.Run(context.Background(), []string{"app", "go", "-n", "abc"})
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Error(), "integer") {
		t.Fatalf("err = %v, want integer ValidationError", err)
	}
}

// Multiple validators on one flag run in order; the first failure wins.
func TestValidator_MultiplePerFlag(t *testing.T) {
	var ran bool
	app := NewApp("app")
	app.Add(Command{
		Name: "go",
		Flags: func(fs *flag.FlagSet) {
			fs.String("token", "", "token")
			RegisterValidator(fs, "token", Required())
			RegisterValidator(fs, "token", Predicate(
				func(v string) bool { return strings.HasPrefix(v, "t-") },
				"must start with t-",
			))
		},
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error { ran = true; return nil },
	})

	// Empty trips Required (first validator).
	err := app.Run(context.Background(), []string{"app", "go", "-token", ""})
	var ve *ValidationError
	if !errors.As(err, &ve) || !strings.Contains(ve.Error(), "empty") {
		t.Fatalf("err = %v, want empty-required error", err)
	}

	// Non-empty but wrong prefix trips the predicate (second validator).
	err = app.Run(context.Background(), []string{"app", "go", "-token", "nope"})
	if !errors.As(err, &ve) || !strings.Contains(ve.Error(), "t-") {
		t.Fatalf("err = %v, want prefix error", err)
	}

	// Valid value passes both.
	if err := app.Run(context.Background(), []string{"app", "go", "-token", "t-abc"}); err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	if !ran {
		t.Errorf("command did not run for valid token")
	}
}

// ValidationError unwraps to the underlying validator error.
func TestValidationError_Unwrap(t *testing.T) {
	underlying := errors.New("nope")
	ve := &ValidationError{Flag: "x", Value: "v", Err: underlying}
	if !errors.Is(ve, underlying) {
		t.Errorf("errors.Is(ve, underlying) = false")
	}
}

// RegisterValidator outside a Flags callback is a harmless no-op.
func TestRegisterValidator_NoActiveRegistry(t *testing.T) {
	fs := flag.NewFlagSet("x", flag.ContinueOnError)
	fs.String("a", "", "")
	// No active registry installed for fs — must not panic.
	RegisterValidator(fs, "a", Required())
}

func TestValidators_Standalone(t *testing.T) {
	tests := []struct {
		name    string
		v       Validator
		value   string
		wantErr bool
	}{
		{"required empty", Required(), "", true},
		{"required spaces", Required(), "   ", true},
		{"required ok", Required(), "x", false},
		{"oneof miss", OneOf("a", "b"), "c", true},
		{"oneof hit", OneOf("a", "b"), "b", false},
		{"range below", Range(1, 10), "0", true},
		{"range above", Range(1, 10), "11", true},
		{"range nonint", Range(1, 10), "x", true},
		{"range ok", Range(1, 10), "5", false},
		{"url empty", NonEmptyURL(), "", true},
		{"url relative", NonEmptyURL(), "/path", true},
		{"url ok", NonEmptyURL(), "https://api.akeyless.io", false},
		{"predicate fail", Predicate(func(string) bool { return false }, "no"), "x", true},
		{"predicate ok", Predicate(func(string) bool { return true }, "no"), "x", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.v(tc.value)
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
