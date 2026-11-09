package cli

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/witnsby/aws-sso-login/src/internal/profiles"
)

var pickerFixture = []profiles.Profile{
	{Name: "dev", AccountID: "123456789012", RoleName: "AdministratorAccess"},
	{Name: "hipaa-vg-prod-ro", AccountID: "210987654321", RoleName: "ReadOnlyAccess"},
	{Name: "hipaa-vg-prod-rw", AccountID: "210987654321", RoleName: "PowerUserAccess"},
	{Name: "session-dev", AccountID: "111122223333", RoleName: "AdministratorAccess", SessionName: "corp"},
}

// renderPicker drives the picker form like a terminal would: a window size,
// then one key message per typed rune. It returns the rendered view.
func renderPicker(t *testing.T, p []profiles.Profile, typed string) string {
	t.Helper()
	var choice string
	var m tea.Model = newProfileForm(p, &choice)
	m.Init()
	m, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	for _, r := range typed {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m.View()
}

// listed reports which fixture profiles appear as rows in the view.
func listed(view string) []string {
	var names []string
	for _, line := range strings.Split(view, "\n") {
		fields := strings.Fields(strings.Trim(line, "┃> "))
		if len(fields) == 3 {
			names = append(names, fields[0])
		}
	}
	return names
}

func TestProfilePickerFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		typed string
		want  []string
	}{
		{"no filter lists all", "", []string{"dev", "hipaa-vg-prod-ro", "hipaa-vg-prod-rw", "session-dev"}},
		{"name fragment", "vg-prod", []string{"hipaa-vg-prod-ro", "hipaa-vg-prod-rw"}},
		{"account ID fragment", "1234567", []string{"dev"}},
		{"role fragment ignores case", "readonly", []string{"hipaa-vg-prod-ro"}},
		{"legacy and sso-session profiles", "dev", []string{"dev", "session-dev"}},
		{"no match", "xyz", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			view := renderPicker(t, pickerFixture, tc.typed)
			assert.Equal(t, tc.want, listed(view))
			assert.Contains(t, view, pickerHint)
			assert.Contains(t, view, "/"+tc.typed, "filter input should be focused on open")
		})
	}
}

func TestProfilePickerHeight(t *testing.T) {
	t.Parallel()

	many := make([]profiles.Profile, 40)
	for i := range many {
		many[i] = profiles.Profile{Name: "profile-" + string(rune('a'+i%26)) + string(rune('a'+i/26)), AccountID: "123456789012", RoleName: "ReadOnlyAccess"}
	}
	rows := listed(renderPicker(t, many, ""))
	assert.Len(t, rows, pickerHeight-2, "input and hint take two of the picker lines")
}
