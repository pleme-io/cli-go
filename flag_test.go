package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

// A typed string flag binds, parses, and reads back through Get.
func TestFlag_StringBindAndGet(t *testing.T) {
	mode := NewFlag[string]("mode", "fast", "execution mode")
	var got string
	app := NewApp("app")
	app.Add(Command{
		Name:  "run",
		Flags: func(fs *flag.FlagSet) { mode.Bind(fs) },
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
			got = mode.Get()
			return nil
		},
	})
	if err := app.Run(context.Background(), []string{"app", "run", "-mode", "slow"}); err != nil {
		t.Fatal(err)
	}
	if got != "slow" {
		t.Errorf("Get() = %q, want slow", got)
	}
}

// Default is returned when the flag is not given.
func TestFlag_Default(t *testing.T) {
	port := NewFlag[int]("port", 8080, "listen port")
	var got int
	app := NewApp("app")
	app.Add(Command{
		Name:  "serve",
		Flags: func(fs *flag.FlagSet) { port.Bind(fs) },
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
			got = port.Get()
			return nil
		},
	})
	if err := app.Run(context.Background(), []string{"app", "serve"}); err != nil {
		t.Fatal(err)
	}
	if got != 8080 {
		t.Errorf("Get() = %d, want 8080 (default)", got)
	}
}

// OneOf validates AND auto-documents (Law 4): a bad value is rejected with a
// typed ValidationError, and the usage string lists the allowed set.
func TestFlag_OneOf_ValidatesAndDocuments(t *testing.T) {
	mode := NewFlag[string]("mode", "fast", "execution mode").OneOf("fast", "slow")
	var buf bytes.Buffer
	app := NewApp("app", WithOutput(&buf))
	app.Add(Command{
		Name:  "run",
		Flags: func(fs *flag.FlagSet) { mode.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})

	// Bad enum value → typed ValidationError on the right flag.
	err := app.Run(context.Background(), []string{"app", "run", "-mode", "weird"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Flag != "mode" {
		t.Fatalf("err = %v, want *ValidationError for mode", err)
	}
	if !strings.Contains(ve.Error(), "fast") || !strings.Contains(ve.Error(), "slow") {
		t.Errorf("error %q should list allowed set", ve.Error())
	}

	// Help DERIVES the allowed set from the OneOf data, not hand-formatting.
	buf.Reset()
	_ = app.Run(context.Background(), []string{"app", "run", "--help"})
	out := buf.String()
	if !strings.Contains(out, "one of: fast, slow") {
		t.Errorf("usage should auto-document OneOf set:\n%s", out)
	}
}

// A valid OneOf value passes.
func TestFlag_OneOf_Passes(t *testing.T) {
	mode := NewFlag[string]("mode", "fast", "mode").OneOf("fast", "slow")
	app := NewApp("app")
	app.Add(Command{
		Name:  "run",
		Flags: func(fs *flag.FlagSet) { mode.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})
	if err := app.Run(context.Background(), []string{"app", "run", "-mode", "slow"}); err != nil {
		t.Errorf("valid value rejected: %v", err)
	}
}

// The typed Validate hook receives the typed value (int, not string).
func TestFlag_TypedValidate(t *testing.T) {
	port := NewFlag[int]("port", 8080, "port").Validate(func(v int) error {
		if v < 1 || v > 65535 {
			return errors.New("out of range")
		}
		return nil
	})
	app := NewApp("app")
	app.Add(Command{
		Name:  "serve",
		Flags: func(fs *flag.FlagSet) { port.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})
	err := app.Run(context.Background(), []string{"app", "serve", "-port", "70000"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Flag != "port" {
		t.Fatalf("err = %v, want ValidationError for port", err)
	}
	if !strings.Contains(ve.Error(), "out of range") {
		t.Errorf("err = %v, want range message", ve.Error())
	}
}

// Env seeds the default and is auto-documented in usage.
func TestFlag_Env(t *testing.T) {
	t.Setenv("TOOL_MODE", "slow")
	mode := NewFlag[string]("mode", "fast", "mode").Env("TOOL_MODE")
	var got string
	var buf bytes.Buffer
	app := NewApp("app", WithOutput(&buf))
	app.Add(Command{
		Name:  "run",
		Flags: func(fs *flag.FlagSet) { mode.Bind(fs) },
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
			got = mode.Get()
			return nil
		},
	})
	// No flag given → env seeds it.
	if err := app.Run(context.Background(), []string{"app", "run"}); err != nil {
		t.Fatal(err)
	}
	if got != "slow" {
		t.Errorf("Get() = %q, want slow (from env)", got)
	}
	// Explicit flag wins over env.
	if err := app.Run(context.Background(), []string{"app", "run", "-mode", "fast"}); err != nil {
		t.Fatal(err)
	}
	if got != "fast" {
		t.Errorf("Get() = %q, want fast (flag overrides env)", got)
	}
	// Usage documents the env name.
	buf.Reset()
	_ = app.Run(context.Background(), []string{"app", "run", "--help"})
	if !strings.Contains(buf.String(), "[env: TOOL_MODE]") {
		t.Errorf("usage should document env name:\n%s", buf.String())
	}
}

// A named string type (type Mode string) binds and reads through the namedValue
// path.
type runMode string

func TestFlag_NamedStringType(t *testing.T) {
	mode := NewFlag[runMode]("mode", runMode("fast"), "mode").OneOf(runMode("fast"), runMode("slow"))
	var got runMode
	app := NewApp("app")
	app.Add(Command{
		Name:  "run",
		Flags: func(fs *flag.FlagSet) { mode.Bind(fs) },
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
			got = mode.Get()
			return nil
		},
	})
	if err := app.Run(context.Background(), []string{"app", "run", "-mode", "slow"}); err != nil {
		t.Fatal(err)
	}
	if got != runMode("slow") {
		t.Errorf("Get() = %q, want slow", got)
	}
	// Bad value rejected via OneOf on the named type.
	err := app.Run(context.Background(), []string{"app", "run", "-mode", "weird"})
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
}

// A bool flag with presence-semantics env (NO_COLOR convention): the variable
// being set to any value enables the flag.
func TestFlag_BoolEnvPresence(t *testing.T) {
	t.Setenv("NO_COLOR", "") // present but empty → presence semantics → true
	noColor := NewFlag[bool]("no-color", false, "disable color").Env("NO_COLOR")
	var got bool
	app := NewApp("app")
	app.Add(Command{
		Name:  "run",
		Flags: func(fs *flag.FlagSet) { noColor.Bind(fs) },
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
			got = noColor.Get()
			return nil
		},
	})
	if err := app.Run(context.Background(), []string{"app", "run"}); err != nil {
		t.Fatal(err)
	}
	if !got {
		t.Errorf("no-color = false, want true (NO_COLOR present)")
	}
}

// Get before Bind/parse returns the default safely.
func TestFlag_GetBeforeBind(t *testing.T) {
	f := NewFlag[int]("x", 42, "x")
	if f.Get() != 42 {
		t.Errorf("Get() before bind = %d, want 42 (default)", f.Get())
	}
}
