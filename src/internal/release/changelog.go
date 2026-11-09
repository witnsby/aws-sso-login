package release

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// PromoteOptions describes the release a changelog is promoted for.
type PromoteOptions struct {
	// Version is the new tag, e.g. "v0.4.0".
	Version string
	// Previous is the previous tag, or "" for the first release.
	Previous string
	// Date is the release date as YYYY-MM-DD.
	Date string
	// RepoURL is the repository web URL used in compare links.
	RepoURL string
	// Commits are the commits in the release, used when [Unreleased] is
	// empty.
	Commits []Commit
}

const unreleasedHeading = "## [Unreleased]"

var (
	sectionHeading = regexp.MustCompile(`^## \[`)
	linkReference  = regexp.MustCompile(`^\[[^\]]+\]: `)
	datePattern    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// PromoteChangelog moves the content of the [Unreleased] section of a Keep a
// Changelog document into a new "## [Version] - Date" section, leaves an
// empty [Unreleased] section and updates the compare links. When
// [Unreleased] is empty, entries are generated from opts.Commits.
func PromoteChangelog(content string, opts PromoteOptions) (string, error) {
	if _, err := ParseVersion(opts.Version); err != nil {
		return "", err
	}
	if !datePattern.MatchString(opts.Date) {
		return "", fmt.Errorf("invalid release date %q: want YYYY-MM-DD", opts.Date)
	}
	if strings.Contains(content, "\n## ["+opts.Version+"]") {
		return "", fmt.Errorf("changelog already has a section for %s", opts.Version)
	}

	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == unreleasedHeading {
			start = i
			break
		}
	}
	if start < 0 {
		return "", errors.New("changelog has no \"## [Unreleased]\" section")
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if sectionHeading.MatchString(lines[i]) || linkReference.MatchString(lines[i]) {
			end = i
			break
		}
	}

	body := trimBlankLines(lines[start+1 : end])
	if len(body) == 0 {
		body = generatedEntries(opts.Commits)
	}
	if len(body) == 0 {
		return "", fmt.Errorf("nothing to add to the changelog for %s: [Unreleased] is empty and no commit is releasable", opts.Version)
	}

	out := make([]string, 0, len(lines)+len(body)+6)
	out = append(out, lines[:start+1]...)
	out = append(out, "", fmt.Sprintf("## [%s] - %s", opts.Version, opts.Date), "")
	out = append(out, body...)
	out = append(out, "")
	out = append(out, lines[end:]...)

	return updateLinks(strings.Join(out, "\n"), opts), nil
}

func trimBlankLines(lines []string) []string {
	s, e := 0, len(lines)
	for s < e && strings.TrimSpace(lines[s]) == "" {
		s++
	}
	for e > s && strings.TrimSpace(lines[e-1]) == "" {
		e--
	}
	return lines[s:e]
}

// generatedEntries renders Keep a Changelog subsections from commits:
// features under Added, breaking changes, perf and revert under Changed,
// fixes under Fixed.
func generatedEntries(commits []Commit) []string {
	var added, changed, fixed []string
	for _, c := range commits {
		if !c.Releasing() {
			continue
		}
		switch {
		case c.Breaking:
			changed = append(changed, "- **BREAKING**: "+c.Text())
		case c.Type == "feat":
			added = append(added, "- "+c.Text())
		case c.Type == "fix":
			fixed = append(fixed, "- "+c.Text())
		default:
			changed = append(changed, "- "+c.Text())
		}
	}
	var out []string
	for _, group := range []struct {
		title   string
		entries []string
	}{{"Added", added}, {"Changed", changed}, {"Fixed", fixed}} {
		if len(group.entries) == 0 {
			continue
		}
		if len(out) > 0 {
			out = append(out, "")
		}
		out = append(out, "### "+group.title, "")
		out = append(out, group.entries...)
	}
	return out
}

// updateLinks points the [Unreleased] link at the new tag and adds a link for
// the new version right below it. Without an [Unreleased] link, both are
// appended at the end.
func updateLinks(content string, opts PromoteOptions) string {
	repo := strings.TrimRight(opts.RepoURL, "/")
	unreleased := fmt.Sprintf("[Unreleased]: %s/compare/%s...HEAD", repo, opts.Version)
	versionLink := fmt.Sprintf("[%s]: %s/releases/tag/%s", opts.Version, repo, opts.Version)
	if opts.Previous != "" {
		versionLink = fmt.Sprintf("[%s]: %s/compare/%s...%s", opts.Version, repo, opts.Previous, opts.Version)
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "[Unreleased]: ") {
			out := make([]string, 0, len(lines)+1)
			out = append(out, lines[:i]...)
			out = append(out, unreleased, versionLink)
			out = append(out, lines[i+1:]...)
			return strings.Join(out, "\n")
		}
	}
	trimmed := strings.TrimRight(content, "\n")
	return trimmed + "\n\n" + unreleased + "\n" + versionLink + "\n"
}
