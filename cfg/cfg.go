// Package cfg is the cli-go ↔ shikumi-go adapter leaf (Law 8): it provides the
// ONE config-consuming entry a CLI uses, wiring the canonical shikumi loader to
// a primitive's canonical [FromConfig] constructor. The cli-go core stays
// zero-dependency; shikumi-go is imported only here, and tools import this leaf
// — never the cores into each other.
//
// # Canonical shape (§3.5)
//
// WithConfig is the single place the two canonical verbs compose:
//
//	Load config (once, at main):  shikumi.For[Root](name)…Load(ctx)   (shikumi owns)
//	Consume config (every primitive): pkg.FromConfig(cfg pkg.Sub) (*T, error)
//
// WithConfig internally calls shikumi.For, runs the canonical precedence
// pipeline (args > env > file), selects the primitive's sub-struct out of the
// loaded root, and hands THAT sub-struct to the consumer's FromConfig. It
// "kills WithConfig[T] as a separate loader idiom" (§3.5): there is exactly one
// loader (shikumi.For) and exactly one consumer shape (FromConfig); this leaf is
// the wire between them, not a third idiom.
//
//	type Root struct {
//		Logging logging.Config `yaml:"logging"`
//	}
//
//	build := cfg.WithConfig(
//		"akeyless-foo",                       // app name (shikumi discovery)
//		Root{},                               // typed defaults
//		func(r Root) logging.Config { return r.Logging }, // select sub-struct
//		logging.FromConfig,                   // the primitive's FromConfig
//		cfg.WithEnvPrefix("AKL_FOO_"),
//	)
//	log, err := build(ctx) // shikumi.For → select → logging.FromConfig
package cfg

import (
	"context"

	"github.com/pleme-io/shikumi-go"
)

// Option configures the shikumi loader WithConfig builds. These forward to the
// shikumi fluent loader so the precedence pipeline stays shikumi-owned (§3.5);
// cli/cfg adds no config semantics of its own.
type Option struct {
	envPrefix   string
	envOverride string
	dirs        []string
	validator   shikumi.Validator
	secrets     []shikumi.SecretResolver
}

// OptionFunc mutates an Option (functional-options over the shikumi loader).
type OptionFunc func(*Option)

// WithEnvPrefix sets the PREFIX_ used by shikumi's env layer.
func WithEnvPrefix(p string) OptionFunc { return func(o *Option) { o.envPrefix = p } }

// WithEnvOverride names the env var whose value, if set, is the exact config
// path (skips discovery).
func WithEnvOverride(name string) OptionFunc { return func(o *Option) { o.envOverride = name } }

// WithDirs appends extra shikumi discovery directories.
func WithDirs(dirs ...string) OptionFunc {
	return func(o *Option) { o.dirs = append(o.dirs, dirs...) }
}

// WithValidator registers a shikumi validator run during Load.
func WithValidator(v shikumi.Validator) OptionFunc { return func(o *Option) { o.validator = v } }

// WithSecrets registers shikumi secret resolvers for the load.
func WithSecrets(rs ...shikumi.SecretResolver) OptionFunc {
	return func(o *Option) { o.secrets = append(o.secrets, rs...) }
}

// WithConfig returns a builder that, when called, runs the canonical shikumi
// load for the named app, selects the primitive's sub-struct from the loaded
// root via sel, and constructs the primitive through its canonical FromConfig.
// This is the ONLY sanctioned config path for a CLI (§3.5): one loader
// (shikumi.For), one consumer (FromConfig), wired here.
//
// Root is the whole-tool config struct (yaml-tagged) shikumi discovers and
// loads; Sub is the primitive's own config sub-struct (e.g. logging.Config);
// T is the constructed runtime object. FromConfig MUST NOT itself call shikumi
// — it takes the already-loaded sub-struct, exactly as §3.5 requires.
func WithConfig[Root any, Sub any, T any](
	app string,
	defaults Root,
	sel func(Root) Sub,
	fromConfig func(Sub) (*T, error),
	opts ...OptionFunc,
) func(ctx context.Context) (*T, error) {
	o := &Option{}
	for _, f := range opts {
		f(o)
	}
	return func(ctx context.Context) (*T, error) {
		root, err := loadRoot(ctx, app, defaults, o)
		if err != nil {
			return nil, err
		}
		return fromConfig(sel(root))
	}
}

// Load runs the canonical shikumi load for app and returns the whole typed root
// config. Use it when a tool needs the full config tree (e.g. to fan a
// sub-struct into more than one primitive's FromConfig); the precedence
// pipeline (args > env > file) is shikumi's, not duplicated here.
func Load[Root any](ctx context.Context, app string, defaults Root, opts ...OptionFunc) (Root, error) {
	o := &Option{}
	for _, f := range opts {
		f(o)
	}
	return loadRoot(ctx, app, defaults, o)
}

// loadRoot is the single shikumi.For invocation shared by WithConfig and Load,
// so the loader is configured one way (§3.5: one loader shape).
func loadRoot[Root any](ctx context.Context, app string, defaults Root, o *Option) (Root, error) {
	l := shikumi.For[Root](app).Defaults(defaults)
	if o.envPrefix != "" {
		l = l.EnvPrefix(o.envPrefix)
	}
	if o.envOverride != "" {
		l = l.EnvOverride(o.envOverride)
	}
	if len(o.dirs) > 0 {
		l = l.Dirs(o.dirs...)
	}
	if o.validator != nil {
		l = l.Validate(o.validator)
	}
	if len(o.secrets) > 0 {
		l = l.Secrets(o.secrets...)
	}
	return l.Load(ctx)
}
