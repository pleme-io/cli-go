package cli

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Required rejects an empty (or whitespace-only) flag value. Useful for flags
// that have no sensible default but must be set explicitly.
func Required() Validator {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("must not be empty")
		}
		return nil
	}
}

// OneOf rejects any value not in the allowed set (case-sensitive). The allowed
// values are listed in the error message.
func OneOf(allowed ...string) Validator {
	set := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		set[a] = struct{}{}
	}
	return func(value string) error {
		if _, ok := set[value]; !ok {
			return fmt.Errorf("must be one of [%s]", strings.Join(allowed, ", "))
		}
		return nil
	}
}

// Range rejects an integer flag value outside the inclusive [min, max] bounds.
// It also rejects values that do not parse as integers, so it doubles as an
// int-shape check on string flags.
func Range(min, max int) Validator {
	return func(value string) error {
		n, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("must be an integer")
		}
		if n < min || n > max {
			return fmt.Errorf("must be in [%d, %d]", min, max)
		}
		return nil
	}
}

// NonEmptyURL rejects a value that does not parse as an absolute URL with a
// scheme and host (e.g. https://api.akeyless.io).
func NonEmptyURL() Validator {
	return func(value string) error {
		u, err := url.Parse(value)
		if err != nil {
			return fmt.Errorf("must be a valid URL: %v", err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("must be an absolute URL with scheme and host")
		}
		return nil
	}
}

// Predicate adapts an arbitrary bool-returning check into a Validator, using msg
// as the rejection message. It is the escape hatch for one-off rules that do
// not warrant their own constructor.
func Predicate(ok func(value string) bool, msg string) Validator {
	return func(value string) error {
		if !ok(value) {
			return fmt.Errorf("%s", msg)
		}
		return nil
	}
}
