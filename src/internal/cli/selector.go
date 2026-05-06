package cli

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

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

// Pick presents a single-select prompt listing the supplied profiles and returns
// the chosen profile name. Returns an error if the list is empty or the user
// cancels the prompt.
func (HuhSelector) Pick(p []profiles.Profile) (string, error) {
	if len(p) == 0 {
		return "", errors.New("no SSO profiles available to select")
	}

	opts := make([]huh.Option[string], 0, len(p))
	for _, prof := range p {
		label := fmt.Sprintf("%-30s  %-12s  %s", prof.Name, prof.AccountID, prof.RoleName)
		opts = append(opts, huh.NewOption(label, prof.Name))
	}

	var choice string
	err := huh.NewSelect[string]().
		Title("Select an AWS SSO profile").
		Options(opts...).
		Value(&choice).
		Run()
	if err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", errors.New("profile selection cancelled")
		}
		return "", fmt.Errorf("running profile selector: %w", err)
	}

	return choice, nil
}
