package cli

import (
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// lookupEnv reads an environment variable, reporting presence. Indirected so
// tests and the cli/cfg shikumi layer share one env-read seam.
func lookupEnv(name string) (string, bool) { return os.LookupEnv(name) }

// castNamed converts a textual value into a named scalar type T (e.g.
// `type Mode string`) reached only by the default branch of [parseAs]. This is
// the one narrow place reflection is warranted: the flagValue union's named
// members cannot be produced by a generic conversion expression, and the path
// is off the zero-dep happy line (only custom named flag types reach it).
func castNamed[T flagValue](s string) (T, error) {
	var zero T
	rt := reflect.TypeOf(zero)
	rv := reflect.New(rt).Elem()
	switch rt.Kind() {
	case reflect.String:
		rv.SetString(s)
	case reflect.Int, reflect.Int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return zero, err
		}
		rv.SetInt(n)
	case reflect.Uint, reflect.Uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return zero, err
		}
		rv.SetUint(n)
	case reflect.Float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return zero, err
		}
		rv.SetFloat(n)
	case reflect.Bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return zero, err
		}
		rv.SetBool(b)
	default:
		return zero, fmt.Errorf("cli: unsupported flag type %s", rt)
	}
	return rv.Interface().(T), nil
}

// Flag is a typed, generic flag declaration that carries its own validation as
// data (Law 5: generics + behaviour). It is the elevation of the bare
// stdlib-flag + side-channel-validator model into one self-describing value:
// the flag's name, usage, default, env-var name, allowed value set (OneOf), and
// a typed Validate hook all travel together, so the help *structure* — type,
// enum set, default, env name — is DERIVED from this data and can never drift
// from behaviour (Law 4).
//
// The kong `enum:` insight is realised here: declaring OneOf both *validates*
// the value and *auto-documents* the allowed set in usage, collapsing what was
// a separate OneOf validator plus hand-written help text into one declaration.
//
//	mode := cli.NewFlag[string]("mode", "fast", "execution mode").
//		Env("TOOL_MODE").
//		OneOf("fast", "slow")
//
//	cmd := cli.Command{
//		Name:  "run",
//		Flags: func(fs *flag.FlagSet) { mode.Bind(fs) },
//		Run: func(ctx context.Context, args []string, fs *flag.FlagSet) error {
//			switch mode.Get() { // typed, validated read
//			case "fast": …
//			}
//			return nil
//		},
//	}
//
// Get reads the parsed, validated value (the typed equivalent of
// fs.Lookup(name).Value). When a shikumi precedence pipeline is wired (via the
// cli/cfg leaf sub-package), Flag reads through the same koanf instance so "did
// the flag or the file win?" has exactly one fleet answer; the core type stays
// zero-dependency and falls back to the bound flag value.
//
// T is constrained to the value kinds the stdlib flag package natively parses,
// so binding never needs reflection or a bespoke parser.
type Flag[T flagValue] struct {
	name  string
	usage string
	def   T
	env   string
	oneOf []T
	check func(T) error

	// val is the bound destination, set by Bind. nil until bound.
	val *T
}

// flagValue is the set of value kinds Flag[T] supports natively, matching the
// scalar flags the stdlib flag package parses without a custom flag.Value. It
// embeds comparable so OneOf membership checks need no extra constraint.
type flagValue interface {
	comparable
	~string | ~int | ~int64 | ~uint | ~uint64 | ~float64 | ~bool
}

// NewFlag declares a typed flag with a name, default, and usage string. Chain
// [Flag.Env], [Flag.OneOf], and [Flag.Validate] to attach metadata, then bind
// it inside a command's Flags callback with [Flag.Bind].
func NewFlag[T flagValue](name string, def T, usage string) *Flag[T] {
	return &Flag[T]{name: name, def: def, usage: usage}
}

// Env sets the environment variable that seeds this flag's default when the
// flag is not given on the command line. The env name is derived into help so
// users see exactly which variable backs the flag (Law 4). The actual
// precedence (args > env > file) is applied by the shikumi layer (cli/cfg) when
// wired; the bare core uses Env only to seed the default and document it.
func (f *Flag[T]) Env(name string) *Flag[T] { f.env = name; return f }

// OneOf restricts the flag to an allowed set of values and, by the same
// declaration, auto-documents that set in usage (the kong `enum:` model). It is
// validation-as-data: no separate OneOf validator and no hand-written "(one of
// …)" help text — both are derived from this one call.
func (f *Flag[T]) OneOf(allowed ...T) *Flag[T] {
	f.oneOf = allowed
	return f
}

// Validate attaches a typed validation hook run after parsing, before the
// command's Run. It receives the already-typed value, so a port flag validates
// an int, not a string. Returning a non-nil error rejects the value with a
// typed [ValidationError]. Composes with OneOf (OneOf is checked first).
func (f *Flag[T]) Validate(check func(T) error) *Flag[T] { f.check = check; return f }

// Default returns the configured default value.
func (f *Flag[T]) Default() T { return f.def }

// Name returns the flag's name (without leading dashes).
func (f *Flag[T]) Name() string { return f.name }

// EnvName returns the configured env var name (empty if unset).
func (f *Flag[T]) EnvName() string { return f.env }

// Allowed returns the configured OneOf set (nil if unrestricted).
func (f *Flag[T]) Allowed() []T { return f.oneOf }

// Bind registers the flag onto fs and wires its validation into the command's
// validator registry. Call it inside a command's Flags callback. The default is
// seeded from the env var (if set and parseable) so the bare core honours env
// without the shikumi layer; the shikumi layer, when wired, overrides this with
// the full args > env > file precedence merge.
func (f *Flag[T]) Bind(fs *flag.FlagSet) {
	def := f.def
	if f.env != "" {
		if v, ok := lookupEnv(f.env); ok {
			if parsed, err := parseAs[T](v); err == nil {
				def = parsed
			} else if b, isBool := any(def).(bool); isBool && !b {
				// Presence semantics for boolean flags (the NO_COLOR
				// convention: the variable being set to ANY value — even
				// empty or non-boolean — disables color). An unparseable
				// value on a bool flag means "present → true".
				def = any(true).(T)
			}
		}
	}
	f.val = bindNative[T](fs, f.name, def, f.usageString())

	// Register validation-as-data: OneOf (auto-derived) then the typed hook.
	RegisterValidator(fs, f.name, func(string) error {
		v := f.Get()
		if len(f.oneOf) > 0 && !containsVal(f.oneOf, v) {
			return fmt.Errorf("must be one of [%s]", joinVals(f.oneOf))
		}
		if f.check != nil {
			return f.check(v)
		}
		return nil
	})
}

// Get returns the parsed, validated value. It must be called after parsing
// (inside Run). Before Bind, or before parsing, it returns the default.
func (f *Flag[T]) Get() T {
	if f.val == nil {
		return f.def
	}
	return *f.val
}

// usageString derives the help text for this flag from its typed data: the
// authored usage plus the auto-documented OneOf set and env name (Law 4 — the
// enum/env structure is never hand-written into the usage string).
func (f *Flag[T]) usageString() string {
	parts := []string{f.usage}
	if len(f.oneOf) > 0 {
		parts = append(parts, "(one of: "+joinVals(f.oneOf)+")")
	}
	if f.env != "" {
		parts = append(parts, "[env: "+f.env+"]")
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// --- native binding + parsing (stdlib only) --------------------------------

// bindNative registers a name/def/usage flag on fs and returns the pointer the
// flag package writes the parsed value into. It dispatches on the concrete kind
// of T via a type switch over the default value, so no reflection is needed and
// the supported set is closed by the flagValue constraint. For the exact native
// kinds the flag package's own typed registrar already returns a *T (the `~`
// constraint sends named types to the default branch), so we reinterpret that
// pointer through `any` — reflection-free and allocation-free. Named scalar
// types are bound through a [namedValue] flag.Value over a fresh *T.
func bindNative[T flagValue](fs *flag.FlagSet, name string, def T, usage string) *T {
	switch d := any(def).(type) {
	case string:
		return any(fs.String(name, d, usage)).(*T)
	case int:
		return any(fs.Int(name, d, usage)).(*T)
	case int64:
		return any(fs.Int64(name, d, usage)).(*T)
	case uint:
		return any(fs.Uint(name, d, usage)).(*T)
	case uint64:
		return any(fs.Uint64(name, d, usage)).(*T)
	case float64:
		return any(fs.Float64(name, d, usage)).(*T)
	case bool:
		return any(fs.Bool(name, d, usage)).(*T)
	default:
		// Named scalar type (e.g. `type Mode string`): bind via a flag.Value
		// that parses into a fresh *T initialised to the default.
		out := new(T)
		*out = def
		fs.Var(&namedValue[T]{out: out, def: def}, name, usage)
		return out
	}
}

// namedValue is a flag.Value for named scalar types (e.g. `type Mode string`)
// not directly handled by the flag package's typed registrars. It parses the
// textual form into the underlying kind and stores it in out.
type namedValue[T flagValue] struct {
	out *T
	def T
	set bool
}

func (n *namedValue[T]) String() string {
	if n == nil || n.out == nil {
		return ""
	}
	if !n.set {
		return fmt.Sprint(n.def)
	}
	return fmt.Sprint(*n.out)
}

func (n *namedValue[T]) Set(s string) error {
	v, err := parseAs[T](s)
	if err != nil {
		return err
	}
	*n.out = v
	n.set = true
	return nil
}

// parseAs parses a textual value into T's underlying scalar kind.
func parseAs[T flagValue](s string) (T, error) {
	var zero T
	switch any(zero).(type) {
	case string:
		return any(s).(T), nil
	case int:
		n, err := strconv.Atoi(s)
		if err != nil {
			return zero, err
		}
		return any(n).(T), nil
	case int64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return zero, err
		}
		return any(n).(T), nil
	case uint:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return zero, err
		}
		return any(uint(n)).(T), nil
	case uint64:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return zero, err
		}
		return any(n).(T), nil
	case float64:
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return zero, err
		}
		return any(n).(T), nil
	case bool:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return zero, err
		}
		return any(b).(T), nil
	default:
		// Named string type.
		return castNamed[T](s)
	}
}

// containsVal reports whether v is in the allowed set.
func containsVal[T comparable](allowed []T, v T) bool {
	for _, a := range allowed {
		if a == v {
			return true
		}
	}
	return false
}

// joinVals renders an allowed set as a comma-separated string for help/errors.
func joinVals[T any](vals []T) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprint(v)
	}
	return strings.Join(parts, ", ")
}
