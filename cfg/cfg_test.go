package cfg

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// A primitive's config sub-struct and its canonical FromConfig (§3.5):
// FromConfig takes the already-loaded sub-struct and MUST NOT call shikumi.
type logConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type logger struct {
	level  string
	format string
}

func logFromConfig(c logConfig) (*logger, error) {
	return &logger{level: c.Level, format: c.Format}, nil
}

// The whole-tool root config; the primitive's sub-struct is one field.
type rootConfig struct {
	Logging logConfig `yaml:"logging"`
}

// WithConfig runs shikumi.For, selects the sub-struct, and hands it to
// FromConfig — the §3.5 wire. Here defaults + env supply the values (no file).
func TestWithConfig_DefaultsAndEnv(t *testing.T) {
	build := WithConfig(
		"cli-go-cfg-test",
		rootConfig{Logging: logConfig{Level: "info", Format: "json"}},
		func(r rootConfig) logConfig { return r.Logging },
		logFromConfig,
		WithEnvPrefix("CLIGO_CFG_TEST_"),
		WithEnvOverride("CLIGO_CFG_TEST_CONFIG"),
	)

	// Env overrides the default (precedence: env > file/default for unset file).
	t.Setenv("CLIGO_CFG_TEST_LOGGING_LEVEL", "debug")
	l, err := build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if l.level != "debug" {
		t.Errorf("level = %q, want debug (from env)", l.level)
	}
	if l.format != "json" {
		t.Errorf("format = %q, want json (default)", l.format)
	}
}

// A config file is discovered, loaded, decoded into the root, and the
// sub-struct handed to FromConfig.
func TestWithConfig_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("logging:\n  level: warn\n  format: console\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Point the override env at the explicit file (skips XDG discovery).
	t.Setenv("CLIGO_CFG_TEST2_CONFIG", path)

	build := WithConfig(
		"cli-go-cfg-test2",
		rootConfig{Logging: logConfig{Level: "info"}},
		func(r rootConfig) logConfig { return r.Logging },
		logFromConfig,
		WithEnvPrefix("CLIGO_CFG_TEST2_"),
		WithEnvOverride("CLIGO_CFG_TEST2_CONFIG"),
	)
	l, err := build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if l.level != "warn" {
		t.Errorf("level = %q, want warn (from file)", l.level)
	}
	if l.format != "console" {
		t.Errorf("format = %q, want console (from file)", l.format)
	}
}

// Load returns the full typed root for tools that fan a sub-struct into more
// than one primitive's FromConfig.
func TestLoad_FullRoot(t *testing.T) {
	root, err := Load(
		context.Background(),
		"cli-go-cfg-test3",
		rootConfig{Logging: logConfig{Level: "info", Format: "json"}},
		WithEnvPrefix("CLIGO_CFG_TEST3_"),
		WithEnvOverride("CLIGO_CFG_TEST3_CONFIG"),
	)
	if err != nil {
		t.Fatal(err)
	}
	if root.Logging.Level != "info" {
		t.Errorf("level = %q, want info", root.Logging.Level)
	}
}
