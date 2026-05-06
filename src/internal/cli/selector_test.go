package cli

import (
	"errors"
	"testing"

	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// fakeSelector is a deterministic Selector used by tests in this package
// (and by other clusters' wiring tests that need to exercise the import flow
// without spawning the interactive UI). Per Q5 the full huh UI is not unit-tested;
// this fake covers interface wiring only.
type fakeSelector struct {
	choice string
	err    error
}

func (f fakeSelector) Pick(_ []profiles.Profile) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.choice, nil
}

// TestSelectorContract verifies the Selector interface contract:
//   - HuhSelector satisfies Selector (compile-time and runtime).
//   - HuhSelector.Pick returns an error when given an empty profile list.
//   - fakeSelector returns its preset value, demonstrating the seam used by C3.
func TestSelectorContract(t *testing.T) {
	t.Parallel()

	t.Run("HuhSelector satisfies Selector", func(t *testing.T) {
		t.Parallel()
		var s Selector = HuhSelector{}
		if s == nil {
			t.Fatal("HuhSelector{} should satisfy Selector")
		}
	})

	t.Run("HuhSelector rejects empty profile list", func(t *testing.T) {
		t.Parallel()
		_, err := HuhSelector{}.Pick(nil)
		if err == nil {
			t.Fatal("expected error for empty profile list, got nil")
		}
	})

	t.Run("fakeSelector returns preset choice", func(t *testing.T) {
		t.Parallel()
		var s Selector = fakeSelector{choice: "my-profile"}
		got, err := s.Pick([]profiles.Profile{{Name: "ignored"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "my-profile" {
			t.Fatalf("expected %q, got %q", "my-profile", got)
		}
	})

	t.Run("fakeSelector propagates preset error", func(t *testing.T) {
		t.Parallel()
		sentinel := errors.New("cancelled")
		var s Selector = fakeSelector{err: sentinel}
		_, err := s.Pick(nil)
		if !errors.Is(err, sentinel) {
			t.Fatalf("expected sentinel error, got %v", err)
		}
	})
}
