package cli

import (
	"flag"
)

// OutputFormat names a fleet-standard rendering of command output. The set is
// derived once here and auto-documented on the --output flag (Law 4): json and
// yaml are always available; table is opt-in per tool.
type OutputFormat string

const (
	// OutputJSON renders machine-readable JSON (the scripting default).
	OutputJSON OutputFormat = "json"
	// OutputYAML renders YAML.
	OutputYAML OutputFormat = "yaml"
	// OutputTable renders a human-readable table; opt-in per tool via
	// FleetFlags.AllowTable.
	OutputTable OutputFormat = "table"
)

// FleetFlags is the fleet-standard persistent flag set, defined ONCE here so
// every binary inherits the same surface and the same help (Law 1: uniformity;
// Law 4: the --auth/--output value sets are derived, never hand-listed). Bind
// it onto a command's FlagSet with [FleetFlags.Bind] and read the resolved
// values through its typed handles after parsing.
//
//	fleet := cli.NewFleetFlags(resolver) // --auth set derived from resolver.Kinds()
//	root := cli.Command{
//		Name:  "clint",
//		Flags: func(fs *flag.FlagSet) { fleet.Bind(fs) },
//		Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
//			gw := fleet.GatewayURL.Get()
//			return nil
//		},
//	}
//
// The flags are the §2.2 fleet flags: --auth (kinds auto-wired from the
// AuthResolver), --profile, --gateway-url, --output (json|yaml[|table]),
// --no-color (also honouring the NO_COLOR env convention), and -verbose. They
// are "backed, not decorative": --gateway-url > --profile's URL > the default
// is the precedence the auth/config layers consume.
type FleetFlags struct {
	// Auth selects the authentication method; its allowed set is auto-wired from
	// the AuthResolver passed to NewFleetFlags (so --auth documents exactly the
	// methods the tool supports, Law 4 / §2.2).
	Auth *Flag[string]
	// Profile names a saved connection profile (provides a default gateway URL,
	// credentials, etc.).
	Profile *Flag[string]
	// GatewayURL overrides the API endpoint; highest precedence over the
	// profile's URL and the built-in default.
	GatewayURL *Flag[string]
	// Output selects the output rendering (json|yaml, table opt-in).
	Output *Flag[string]
	// NoColor disables styled output; also honours the NO_COLOR env convention
	// via its Env binding.
	NoColor *Flag[bool]
	// Verbose raises log verbosity.
	Verbose *Flag[bool]
}

// FleetOption configures the persistent flag set at construction.
type FleetOption func(*fleetConfig)

type fleetConfig struct {
	allowTable bool
	defaultGW  string
	defaultOut string
}

// AllowTable enables "table" as an --output value for this tool (off by default
// so scripting tools advertise only the structured formats).
func AllowTable() FleetOption { return func(c *fleetConfig) { c.allowTable = true } }

// DefaultGatewayURL sets the built-in --gateway-url default (lowest precedence).
func DefaultGatewayURL(u string) FleetOption { return func(c *fleetConfig) { c.defaultGW = u } }

// DefaultOutput sets the default --output format (defaults to json).
func DefaultOutput(f OutputFormat) FleetOption {
	return func(c *fleetConfig) { c.defaultOut = string(f) }
}

// NewFleetFlags builds the persistent flag set, deriving the --auth allowed set
// from resolver.Kinds() (pass nil to leave --auth unrestricted, e.g. for tools
// that resolve auth elsewhere). The returned value holds typed [Flag] handles;
// bind them with Bind and read them after parsing.
func NewFleetFlags(resolver *AuthResolver, opts ...FleetOption) *FleetFlags {
	cfg := &fleetConfig{defaultOut: string(OutputJSON)}
	for _, o := range opts {
		o(cfg)
	}

	auth := NewFlag[string]("auth", "", "authentication method").Env("AKEYLESS_AUTH")
	if resolver != nil {
		if kinds := resolver.Kinds(); len(kinds) > 0 {
			allowed := make([]string, len(kinds))
			for i, k := range kinds {
				allowed[i] = string(k)
			}
			// Allow the empty selector (auto-detect) plus each method.
			auth.OneOf(append([]string{""}, allowed...)...)
		}
	}

	outVals := []OutputFormat{OutputJSON, OutputYAML}
	if cfg.allowTable {
		outVals = append(outVals, OutputTable)
	}
	outAllowed := make([]string, len(outVals))
	for i, v := range outVals {
		outAllowed[i] = string(v)
	}

	return &FleetFlags{
		Auth: auth,
		Profile: NewFlag[string]("profile", "", "named connection profile").
			Env("AKEYLESS_PROFILE"),
		GatewayURL: NewFlag[string]("gateway-url", cfg.defaultGW, "Akeyless API/Gateway URL").
			Env("AKEYLESS_GATEWAY_URL").
			Validate(func(v string) error {
				if v == "" {
					return nil // empty means "use profile/default"
				}
				return NonEmptyURL()(v)
			}),
		Output: NewFlag[string]("output", cfg.defaultOut, "output format").
			Env("AKEYLESS_OUTPUT").
			OneOf(outAllowed...),
		NoColor: NewFlag[bool]("no-color", false, "disable styled/colored output").
			Env("NO_COLOR"),
		Verbose: NewFlag[bool]("verbose", false, "verbose output"),
	}
}

// Bind registers every persistent flag onto fs (call inside a command's Flags
// callback). After parsing, read the resolved values through the typed handles.
func (f *FleetFlags) Bind(fs *flag.FlagSet) {
	f.Auth.Bind(fs)
	f.Profile.Bind(fs)
	f.GatewayURL.Bind(fs)
	f.Output.Bind(fs)
	f.NoColor.Bind(fs)
	f.Verbose.Bind(fs)
}
