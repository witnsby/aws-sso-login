package profiles

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-ini/ini"
)

// Profile holds the SSO-relevant fields for a single AWS named profile.
type Profile struct {
	Name      string // e.g. "hipaa-master"
	AccountID string // sso_account_id
	RoleName  string // sso_role_name
	Region    string // sso_region
	StartURL  string // sso_start_url
}

// ListSSOProfiles parses configPath (e.g. ~/.aws/config), returns only
// profiles that have a non-empty sso_start_url, sorted alphabetically by Name.
// Default profile ([default]) is included if it is SSO-enabled.
func ListSSOProfiles(configPath string) ([]Profile, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{}, configPath)
	if err != nil {
		return nil, fmt.Errorf("loading aws config %s: %w", configPath, err)
	}

	var results []Profile
	for _, section := range cfg.Sections() {
		name, ok := profileName(section.Name())
		if !ok {
			continue
		}

		startURL := section.Key("sso_start_url").String()
		if startURL == "" {
			continue
		}

		results = append(results, Profile{
			Name:      name,
			AccountID: section.Key("sso_account_id").String(),
			RoleName:  section.Key("sso_role_name").String(),
			Region:    section.Key("sso_region").String(),
			StartURL:  startURL,
		})
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no SSO-enabled profiles found in %s", configPath)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return results, nil
}

// profileName returns the logical profile name and true when the section is a
// recognised AWS config section ([default] or [profile <name>]).
// Returns "", false for all other sections (e.g. the synthetic DEFAULT section
// that go-ini always adds).
func profileName(sectionName string) (string, bool) {
	if sectionName == "default" {
		return "default", true
	}
	const prefix = "profile "
	if strings.HasPrefix(sectionName, prefix) {
		name := strings.TrimPrefix(sectionName, prefix)
		if name != "" {
			return name, true
		}
	}
	return "", false
}
