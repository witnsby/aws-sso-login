package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

// errSelectionCancelled is returned by a Selector when the user cancels.
var errSelectionCancelled = errors.New("profile selection cancelled")

// Selector picks an AWS SSO profile from a list. Implementations may render
// an interactive UI or be replaced with a deterministic fake in tests.
type Selector interface {
	// Pick returns the chosen profile name, or an error if cancelled / empty list.
	Pick(profiles []profiles.Profile) (string, error)
}

// HuhSelector is the production implementation backed by github.com/charmbracelet/huh.
type HuhSelector struct{}

// Compile-time assertion that HuhSelector satisfies the Selector interface.
var _ Selector = HuhSelector{}

// pickerHeight caps the picker at this many lines, filter input and hint
// included, so long profile lists scroll instead of filling the terminal.
const pickerHeight = 15

// pickerHint is shown under the filter input, which replaces the title while
// filtering is active.
const pickerHint = "Type to filter by name, account ID or role"

// newProfileForm builds the picker form. It opens with the filter input
// focused; huh keeps the options whose label (name, account ID and role)
// contains the typed text, compared case-insensitively.
func newProfileForm(p []profiles.Profile, choice *string) *huh.Form {
	opts := make([]huh.Option[string], 0, len(p))
	for _, prof := range p {
		label := fmt.Sprintf("%-30s  %-12s  %s", prof.Name, prof.AccountID, prof.RoleName)
		opts = append(opts, huh.NewOption(label, prof.Name))
	}

	sel := huh.NewSelect[string]().
		Title("Select an AWS SSO profile").
		Description(pickerHint).
		Options(opts...).
		Filtering(true).
		Height(pickerHeight).
		Value(choice)
	return huh.NewForm(huh.NewGroup(sel))
}

// Pick presents a single-select prompt listing the supplied profiles and returns
// the chosen profile name. Returns an error if the list is empty or the user
// cancels the prompt.
func (HuhSelector) Pick(p []profiles.Profile) (string, error) {
	if len(p) == 0 {
		return "", errors.New("no SSO profiles available to select")
	}

	var choice string
	// Render on stderr so `eval "$(aws-sso-login export)"` never captures
	// the picker UI.
	err := newProfileForm(p, &choice).WithOutput(os.Stderr).Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", errSelectionCancelled
		}
		return "", fmt.Errorf("running profile selector: %w", err)
	}

	return choice, nil
}
