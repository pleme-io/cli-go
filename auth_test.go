package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestAuthResolver_ExplicitSelector(t *testing.T) {
	t.Setenv("AK_KEY", "secret-key")
	r := NewAuthResolver().
		Register(AuthMethod{Kind: AuthAPIKey, EnvVar: "AK_KEY"}).
		Register(AuthMethod{Kind: AuthToken, EnvVar: "AK_TOKEN"})

	res, err := r.Resolve(string(AuthAPIKey))
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != AuthAPIKey {
		t.Errorf("kind = %q, want %q", res.Kind, AuthAPIKey)
	}
	if res.Credentials[string(AuthAPIKey)] != "secret-key" {
		t.Errorf("creds = %v, want api-key=secret-key", res.Credentials)
	}
}

func TestAuthResolver_EnvAutoDetect(t *testing.T) {
	t.Setenv("AK_TOKEN", "tok-123")
	r := NewAuthResolver().
		Register(AuthMethod{Kind: AuthAPIKey, EnvVar: "AK_KEY"}).
		Register(AuthMethod{Kind: AuthToken, EnvVar: "AK_TOKEN"})

	res, err := r.Resolve("") // no explicit selector
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != AuthToken {
		t.Errorf("kind = %q, want auto-detected token", res.Kind)
	}
	if res.Credentials[string(AuthToken)] != "tok-123" {
		t.Errorf("creds = %v", res.Credentials)
	}
}

func TestAuthResolver_UnknownSelector(t *testing.T) {
	r := NewAuthResolver().Register(AuthMethod{Kind: AuthAPIKey, EnvVar: "AK_KEY"})
	_, err := r.Resolve("saml")
	if err == nil || !strings.Contains(err.Error(), "unknown auth method") {
		t.Fatalf("err = %v, want unknown auth method", err)
	}
	if !strings.Contains(err.Error(), string(AuthAPIKey)) {
		t.Errorf("err should list available kinds: %v", err)
	}
}

func TestAuthResolver_NoneSelectedNoEnv(t *testing.T) {
	r := NewAuthResolver().
		Register(AuthMethod{Kind: AuthAPIKey, EnvVar: "AK_KEY_UNSET_XYZ"}).
		Register(AuthMethod{Kind: AuthToken, EnvVar: "AK_TOKEN_UNSET_XYZ"})
	_, err := r.Resolve("")
	if err == nil || !strings.Contains(err.Error(), "none auto-detected") {
		t.Fatalf("err = %v, want none auto-detected", err)
	}
}

func TestAuthResolver_NoMethods(t *testing.T) {
	r := NewAuthResolver()
	if _, err := r.Resolve(""); err == nil || !strings.Contains(err.Error(), "no auth methods registered") {
		t.Fatalf("err = %v, want no methods registered", err)
	}
}

func TestAuthResolver_CustomResolve(t *testing.T) {
	sentinel := errors.New("vault down")
	r := NewAuthResolver().Register(AuthMethod{
		Kind:   AuthAWS,
		EnvVar: "AWS_ROLE",
		Resolve: func(envValue string) (map[string]string, error) {
			if envValue == "" {
				return nil, sentinel
			}
			return map[string]string{"role": envValue, "region": "us-east-1"}, nil
		},
	})

	t.Setenv("AWS_ROLE", "akeyless-reader")
	res, err := r.Resolve(string(AuthAWS))
	if err != nil {
		t.Fatal(err)
	}
	if res.Credentials["role"] != "akeyless-reader" || res.Credentials["region"] != "us-east-1" {
		t.Errorf("creds = %v", res.Credentials)
	}
}

func TestAuthResolver_CustomResolveError(t *testing.T) {
	sentinel := errors.New("vault down")
	r := NewAuthResolver().Register(AuthMethod{
		Kind:    AuthAWS,
		Resolve: func(string) (map[string]string, error) { return nil, sentinel },
	})
	_, err := r.Resolve(string(AuthAWS))
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want wrapped sentinel", err)
	}
}

func TestAuthResolver_KindsSortedAndDeduped(t *testing.T) {
	r := NewAuthResolver().
		Register(AuthMethod{Kind: AuthToken}).
		Register(AuthMethod{Kind: AuthAPIKey}).
		Register(AuthMethod{Kind: AuthToken}) // duplicate

	kinds := r.Kinds()
	want := []AuthKind{AuthAPIKey, AuthToken} // sorted: "api-key" < "token"
	if len(kinds) != len(want) {
		t.Fatalf("kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("kinds[%d] = %q, want %q", i, kinds[i], want[i])
		}
	}
}

// A later registration of the same kind shadows the earlier one.
func TestAuthResolver_LastRegistrationWins(t *testing.T) {
	r := NewAuthResolver().
		Register(AuthMethod{Kind: AuthAPIKey, EnvVar: "FIRST"}).
		Register(AuthMethod{Kind: AuthAPIKey, EnvVar: "SECOND"})
	t.Setenv("SECOND", "v")
	res, err := r.Resolve(string(AuthAPIKey))
	if err != nil {
		t.Fatal(err)
	}
	if res.Credentials[string(AuthAPIKey)] != "v" {
		t.Errorf("creds = %v, want the second registration's env", res.Credentials)
	}
}
