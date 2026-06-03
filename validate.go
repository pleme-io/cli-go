package cli

import (
	"flag"
	"fmt"
	"sync"
)

// Validator inspects a parsed [flag.Flag] value and returns a non-nil error to
// reject it. Validators are registered per flag inside a command's Flags
// callback via [RegisterValidator] and run after flag parsing, before the
// command's Run.
type Validator func(value string) error

// registry holds the validators registered for one command invocation, keyed by
// flag name. A FlagSet has exactly one registry while its Flags callback runs.
type registry struct {
	fs     *flag.FlagSet
	byName map[string][]Validator
	order  []string
}

func newRegistry(fs *flag.FlagSet) *registry {
	return &registry{fs: fs, byName: map[string][]Validator{}}
}

// add appends a validator for a flag name, preserving first-seen flag order so
// errors are reported deterministically.
func (r *registry) add(name string, v Validator) {
	if _, seen := r.byName[name]; !seen {
		r.order = append(r.order, name)
	}
	r.byName[name] = append(r.byName[name], v)
}

// validate runs every registered validator against the current value of its
// flag, returning the first failure (as a typed *ValidationError).
func (r *registry) validate() error {
	for _, name := range r.order {
		f := r.fs.Lookup(name)
		if f == nil {
			continue
		}
		val := f.Value.String()
		for _, v := range r.byName[name] {
			if err := v(val); err != nil {
				return &ValidationError{Flag: name, Value: val, Err: err}
			}
		}
	}
	return nil
}

// activeRegistries threads the per-invocation registry to [RegisterValidator]
// without changing the Flags callback signature (which takes a bare
// *flag.FlagSet, matching the flag package). The registry is installed for the
// duration of the Flags callback only.
var activeRegistries sync.Map // *flag.FlagSet -> *registry

// withRegistry installs reg as the active registry for fs while fn runs, so
// [RegisterValidator] calls inside fn find it. It is reentrancy-safe across
// concurrent App.Run calls because the map is keyed by FlagSet pointer.
func withRegistry(fs *flag.FlagSet, reg *registry, fn func()) {
	activeRegistries.Store(fs, reg)
	defer activeRegistries.Delete(fs)
	fn()
}

// RegisterValidator attaches a validator to a flag, to run after parsing and
// before the command's Run. Call it from within a command's Flags callback,
// after registering the flag itself:
//
//	Flags: func(fs *flag.FlagSet) {
//		fs.Int("port", 8080, "listen port")
//		cli.RegisterValidator(fs, "port", cli.Range(1, 65535))
//	},
//
// A flag may carry multiple validators; they run in registration order and the
// first failure wins. Registering a validator outside a Flags callback is a
// no-op (there is no active command invocation to attach to).
func RegisterValidator(fs *flag.FlagSet, name string, v Validator) {
	if r, ok := activeRegistries.Load(fs); ok {
		r.(*registry).add(name, v)
	}
}

// ValidationError is the typed error returned when a flag validator rejects a
// value. It carries the offending flag name and value so callers can branch on
// them; the wrapped Err is the validator's own message.
type ValidationError struct {
	Flag  string // flag name (without leading dashes)
	Value string // the value that failed
	Err   error  // the underlying validator error
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("cli: invalid value %q for flag -%s: %v", e.Value, e.Flag, e.Err)
}

// Unwrap exposes the underlying validator error for errors.Is/errors.As.
func (e *ValidationError) Unwrap() error { return e.Err }
