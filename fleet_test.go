package cli

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"strings"
	"testing"
)

// The --auth allowed set is DERIVED from resolver.Kinds() (Law 4 / §2.2): a
// kind the resolver knows is accepted; an unknown method is rejected; the set
// is auto-documented in usage.
func TestFleetFlags_AuthSetFromResolver(t *testing.T) {
	resolver := NewAuthResolver().
		Register(AuthMethod{Kind: AuthAPIKey}).
		Register(AuthMethod{Kind: AuthToken})
	fleet := NewFleetFlags(resolver)

	var buf bytes.Buffer
	app := NewApp("clint", WithOutput(&buf))
	app.Add(Command{
		Name:  "list",
		Flags: func(fs *flag.FlagSet) { fleet.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})

	// A known method passes.
	if err := app.Run(context.Background(), []string{"clint", "list", "-auth", "api-key"}); err != nil {
		t.Errorf("known auth rejected: %v", err)
	}

	// An unknown method is rejected (derived OneOf).
	err := app.Run(context.Background(), []string{"clint", "list", "-auth", "saml"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Flag != "auth" {
		t.Fatalf("err = %v, want ValidationError for auth", err)
	}

	// Usage auto-documents the derived auth set.
	buf.Reset()
	_ = app.Run(context.Background(), []string{"clint", "list", "--help"})
	out := buf.String()
	if !strings.Contains(out, "api-key") || !strings.Contains(out, "token") {
		t.Errorf("usage should document derived auth set:\n%s", out)
	}
}

// The standard fleet flags are all present and read through their handles.
func TestFleetFlags_AllFlagsPresent(t *testing.T) {
	fleet := NewFleetFlags(nil, AllowTable(), DefaultGatewayURL("https://api.akeyless.io"))
	var (
		gw, output, profile string
		verbose, noColor    bool
	)
	app := NewApp("clint")
	app.Add(Command{
		Name:  "go",
		Flags: func(fs *flag.FlagSet) { fleet.Bind(fs) },
		Run: func(ctx context.Context, a []string, fs *flag.FlagSet) error {
			gw = fleet.GatewayURL.Get()
			output = fleet.Output.Get()
			profile = fleet.Profile.Get()
			verbose = fleet.Verbose.Get()
			noColor = fleet.NoColor.Get()
			return nil
		},
	})
	argv := []string{"clint", "go",
		"-gateway-url", "https://gw.example.com",
		"-output", "yaml",
		"-profile", "prod",
		"-verbose",
		"-no-color",
	}
	if err := app.Run(context.Background(), argv); err != nil {
		t.Fatal(err)
	}
	if gw != "https://gw.example.com" {
		t.Errorf("gateway-url = %q", gw)
	}
	if output != "yaml" {
		t.Errorf("output = %q, want yaml", output)
	}
	if profile != "prod" {
		t.Errorf("profile = %q, want prod", profile)
	}
	if !verbose {
		t.Errorf("verbose = false, want true")
	}
	if !noColor {
		t.Errorf("no-color = false, want true")
	}
}

// --output is restricted to the structured formats; table is opt-in.
func TestFleetFlags_OutputTableOptIn(t *testing.T) {
	// Without AllowTable, "table" is rejected.
	fleet := NewFleetFlags(nil)
	app := NewApp("clint")
	app.Add(Command{
		Name:  "go",
		Flags: func(fs *flag.FlagSet) { fleet.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})
	err := app.Run(context.Background(), []string{"clint", "go", "-output", "table"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Flag != "output" {
		t.Fatalf("err = %v, want output rejected without AllowTable", err)
	}

	// With AllowTable, "table" passes.
	fleet2 := NewFleetFlags(nil, AllowTable())
	app2 := NewApp("clint")
	app2.Add(Command{
		Name:  "go",
		Flags: func(fs *flag.FlagSet) { fleet2.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})
	if err := app2.Run(context.Background(), []string{"clint", "go", "-output", "table"}); err != nil {
		t.Errorf("table rejected with AllowTable: %v", err)
	}
}

// An invalid (non-URL) gateway-url is rejected, but the empty value (meaning
// "use profile/default") is accepted.
func TestFleetFlags_GatewayURLValidation(t *testing.T) {
	fleet := NewFleetFlags(nil)
	app := NewApp("clint")
	app.Add(Command{
		Name:  "go",
		Flags: func(fs *flag.FlagSet) { fleet.Bind(fs) },
		Run:   func(ctx context.Context, a []string, fs *flag.FlagSet) error { return nil },
	})
	// empty is fine
	if err := app.Run(context.Background(), []string{"clint", "go"}); err != nil {
		t.Errorf("empty gateway-url rejected: %v", err)
	}
	// garbage rejected
	err := app.Run(context.Background(), []string{"clint", "go", "-gateway-url", "not-a-url"})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Flag != "gateway-url" {
		t.Fatalf("err = %v, want gateway-url ValidationError", err)
	}
}
