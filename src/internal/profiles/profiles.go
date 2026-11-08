package profiles

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-ini/ini"
)

// Profile holds the resolved SSO configuration of a single AWS named profile.
// For sso-session profiles, StartURL and SSORegion come from the referenced
// [sso-session <name>] section.
type Profile struct {
	Name        string // e.g. "dev-account"
	AccountID   string // sso_account_id
	RoleName    string // sso_role_name
	StartURL    string // sso_start_url
	SSORegion   string // sso_region
	Region      string // region (optional, the profile's default region)
	SessionName string // sso_session (empty for legacy profiles)
}

// Resolve loads configPath and returns the validated SSO configuration of the
// named profile. "default" maps to [default], falling back to
// [profile default]; every other name maps to [profile <name>].
func Resolve(configPath, name string) (Profile, error) {
	cfg, err := load(configPath)
	if err != nil {
		return Profile{}, err
	}
	section := findSection(cfg, name)
	if section == nil {
		return Profile{}, fmt.Errorf("profile %q not found in %s", name, configPath)
	}
	return fromSection(cfg, name, section)
}

// ListSSOProfiles parses configPath (e.g. ~/.aws/config) and returns every
// profile that resolves as a legacy or sso-session SSO profile, sorted
// case-insensitively by Name. The [default] profile is included when it is
// SSO-enabled. Sections that are not SSO profiles or fail validation are
// skipped.
func ListSSOProfiles(configPath string) ([]Profile, error) {
	cfg, err := load(configPath)
	if err != nil {
		return nil, err
	}

	var results []Profile
	for _, section := range cfg.Sections() {
		name, ok := profileName(section.Name())
		if !ok || !isSSOSection(section) {
			continue
		}
		// Skip sections shadowed by another section with the same logical
		// name (e.g. [profile default] when [default] exists).
		if findSection(cfg, name) != section {
			continue
		}
		p, err := fromSection(cfg, name, section)
		if err != nil {
			continue
		}
		results = append(results, p)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no SSO-enabled profiles found in %s", configPath)
	}

	sort.Slice(results, func(i, j int) bool {
		return strings.ToLower(results[i].Name) < strings.ToLower(results[j].Name)
	})

	return results, nil
}

func load(configPath string) (*ini.File, error) {
	cfg, err := ini.LoadSources(ini.LoadOptions{}, configPath)
	if err != nil {
		return nil, fmt.Errorf("loading aws config %s: %w", configPath, err)
	}
	return cfg, nil
}

// findSection returns the config section for a profile name, or nil.
func findSection(cfg *ini.File, name string) *ini.Section {
	candidates := []string{"profile " + name}
	if name == "default" {
		candidates = []string{"default", "profile default"}
	}
	for _, c := range candidates {
		if s, err := cfg.GetSection(c); err == nil {
			return s
		}
	}
	return nil
}

func isSSOSection(s *ini.Section) bool {
	return value(s, "sso_start_url") != "" || value(s, "sso_session") != ""
}

// value returns a key's value without creating the key when it is absent.
func value(s *ini.Section, key string) string {
	if !s.HasKey(key) {
		return ""
	}
	return strings.TrimSpace(s.Key(key).String())
}

// fromSection builds and validates a Profile from its config section.
func fromSection(cfg *ini.File, name string, s *ini.Section) (Profile, error) {
	p := Profile{
		Name:        name,
		AccountID:   value(s, "sso_account_id"),
		RoleName:    value(s, "sso_role_name"),
		Region:      value(s, "region"),
		SessionName: value(s, "sso_session"),
	}

	if p.SessionName == "" {
		p.StartURL = value(s, "sso_start_url")
		p.SSORegion = value(s, "sso_region")
		return p, requireKeys(name,
			[2]string{"sso_start_url", p.StartURL},
			[2]string{"sso_account_id", p.AccountID},
			[2]string{"sso_role_name", p.RoleName},
			[2]string{"sso_region", p.SSORegion},
		)
	}

	session, err := cfg.GetSection("sso-session " + p.SessionName)
	if err != nil {
		return Profile{}, fmt.Errorf("sso-session %q referenced by profile %q not found", p.SessionName, name)
	}
	p.StartURL = value(session, "sso_start_url")
	p.SSORegion = value(session, "sso_region")
	for _, key := range []string{"sso_start_url", "sso_region"} {
		sessionValue := value(session, key)
		if sessionValue == "" {
			return Profile{}, fmt.Errorf("missing required attribute %q in sso-session %q", key, p.SessionName)
		}
		if profileValue := value(s, key); profileValue != "" && profileValue != sessionValue {
			return Profile{}, fmt.Errorf("conflicting %s values for profile %q: %q in the profile and %q in sso-session %q",
				key, name, profileValue, sessionValue, p.SessionName)
		}
	}
	return p, requireKeys(name,
		[2]string{"sso_account_id", p.AccountID},
		[2]string{"sso_role_name", p.RoleName},
	)
}

// requireKeys returns an error for the first {key, value} pair whose value
// is empty.
func requireKeys(name string, pairs ...[2]string) error {
	for _, kv := range pairs {
		if kv[1] == "" {
			return fmt.Errorf("missing required attribute %q in profile %s", kv[0], name)
		}
	}
	return nil
}

// profileName returns the logical profile name and true when the section is a
// recognised AWS config section ([default] or [profile <name>]).
// Returns "", false for all other sections (e.g. the synthetic DEFAULT section
// that go-ini always adds, or [sso-session ...]).
func profileName(sectionName string) (string, bool) {
	if sectionName == "default" {
		return "default", true
	}
	const prefix = "profile "
	if strings.HasPrefix(sectionName, prefix) {
		name := strings.TrimSpace(strings.TrimPrefix(sectionName, prefix))
		if name != "" {
			return name, true
		}
	}
	return "", false
}
