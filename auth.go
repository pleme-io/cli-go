package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// AuthKind names a supported authentication method. It mirrors the multi-auth
// surface of the Akeyless CLI (api-key, token, AWS IAM, Azure AD, GCP, K8s,
// …); the set is open — register whatever kinds a tool needs.
type AuthKind string

// Common auth kinds. These are conveniences; AuthKind is a plain string and any
// value may be registered.
const (
	AuthAPIKey AuthKind = "api-key"
	AuthToken  AuthKind = "token"
	AuthAWS    AuthKind = "aws-iam"
	AuthAzure  AuthKind = "azure-ad"
	AuthGCP    AuthKind = "gcp"
	AuthK8s    AuthKind = "k8s"
)

// AuthMethod describes one authentication method: how to recognise that it has
// been selected (a flag value and/or an env var) and how to gather its
// credential material once selected.
//
// The abstraction is deliberately transport-free — it resolves *which* method
// and *what* inputs to use, not how to talk to any server. A tool wires the
// resolved [AuthResult] into its own client.
type AuthMethod struct {
	// Kind is the method's identity (e.g. AuthAPIKey).
	Kind AuthKind
	// EnvVar, if set and present in the environment, marks this method as
	// available and supplies its primary credential as the env var's value.
	EnvVar string
	// Resolve gathers credential material for this method. It receives the
	// value already discovered from EnvVar (empty if none) and returns the
	// final credential map. If nil, a single-entry map keyed by the env value
	// (when present) is used.
	Resolve func(envValue string) (map[string]string, error)
}

// AuthResult is the outcome of [AuthResolver.Resolve]: the selected method's
// kind and its gathered credential material.
type AuthResult struct {
	Kind        AuthKind
	Credentials map[string]string
}

// AuthResolver selects exactly one [AuthMethod] from a registered set, using an
// explicit selector (typically the value of an --auth flag) with a fallback to
// whichever method's env var is present.
//
//	r := cli.NewAuthResolver().
//		Register(cli.AuthMethod{Kind: cli.AuthAPIKey, EnvVar: "AKEYLESS_ACCESS_KEY"}).
//		Register(cli.AuthMethod{Kind: cli.AuthToken, EnvVar: "AKEYLESS_TOKEN"})
//
//	res, err := r.Resolve(string(authFlag)) // authFlag may be ""
type AuthResolver struct {
	methods []AuthMethod
}

// NewAuthResolver creates an empty resolver.
func NewAuthResolver() *AuthResolver { return &AuthResolver{} }

// Register adds an auth method, returning the resolver for chaining. A later
// registration of the same Kind shadows an earlier one.
func (r *AuthResolver) Register(m ...AuthMethod) *AuthResolver {
	r.methods = append(r.methods, m...)
	return r
}

// Kinds returns the registered kinds, sorted, for usage and error messages.
func (r *AuthResolver) Kinds() []AuthKind {
	seen := map[AuthKind]struct{}{}
	var out []AuthKind
	for _, m := range r.methods {
		if _, ok := seen[m.Kind]; ok {
			continue
		}
		seen[m.Kind] = struct{}{}
		out = append(out, m.Kind)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// lookup finds a method by kind (last registration wins).
func (r *AuthResolver) lookup(kind AuthKind) (AuthMethod, bool) {
	for i := len(r.methods) - 1; i >= 0; i-- {
		if r.methods[i].Kind == kind {
			return r.methods[i], true
		}
	}
	return AuthMethod{}, false
}

// Resolve selects and materialises an auth method.
//
// Selection order:
//   - if selector is non-empty, it must name a registered kind (else error);
//   - otherwise, the first registered method whose EnvVar is present in the
//     environment is chosen;
//   - if nothing is selected and no env var matches, an error listing the
//     available kinds is returned.
func (r *AuthResolver) Resolve(selector string) (AuthResult, error) {
	if len(r.methods) == 0 {
		return AuthResult{}, fmt.Errorf("cli: no auth methods registered")
	}

	if selector != "" {
		m, ok := r.lookup(AuthKind(selector))
		if !ok {
			return AuthResult{}, fmt.Errorf("cli: unknown auth method %q (available: %s)", selector, joinKinds(r.Kinds()))
		}
		return materialize(m)
	}

	for _, m := range r.methods {
		if m.EnvVar != "" {
			if _, ok := os.LookupEnv(m.EnvVar); ok {
				return materialize(m)
			}
		}
	}
	return AuthResult{}, fmt.Errorf("cli: no auth method selected and none auto-detected from env (available: %s)", joinKinds(r.Kinds()))
}

// materialize runs a method's Resolve (or the default env-value behaviour).
func materialize(m AuthMethod) (AuthResult, error) {
	envValue := ""
	if m.EnvVar != "" {
		envValue = os.Getenv(m.EnvVar)
	}
	if m.Resolve != nil {
		creds, err := m.Resolve(envValue)
		if err != nil {
			return AuthResult{}, fmt.Errorf("cli: resolve auth %q: %w", m.Kind, err)
		}
		return AuthResult{Kind: m.Kind, Credentials: creds}, nil
	}
	creds := map[string]string{}
	if envValue != "" {
		creds[string(m.Kind)] = envValue
	}
	return AuthResult{Kind: m.Kind, Credentials: creds}, nil
}

// joinKinds renders kinds as a comma-separated list.
func joinKinds(kinds []AuthKind) string {
	parts := make([]string, len(kinds))
	for i, k := range kinds {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}
