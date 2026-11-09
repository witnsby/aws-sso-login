package release

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// NotesInput is everything the release description is built from.
type NotesInput struct {
	Version  string
	Previous string // "" for the first release
	RepoURL  string
	Commits  []Commit
	PRs      []PullRequest
}

// RenderNotes renders the release description: breaking changes, features,
// fixes, improvements, other pull requests, closed issues and a compare
// link, leaving out empty sections.
func RenderNotes(in NotesInput) string {
	repo := strings.TrimRight(in.RepoURL, "/")
	prByCommit := map[string]*PullRequest{}
	prByNumber := map[int]*PullRequest{}
	for i := range in.PRs {
		pr := &in.PRs[i]
		prByNumber[pr.Number] = pr
		for _, oid := range pr.Commits {
			if _, taken := prByCommit[oid]; !taken {
				prByCommit[oid] = pr
			}
		}
	}
	prFor := func(c Commit) *PullRequest {
		if pr, ok := prByCommit[c.Hash]; ok {
			return pr
		}
		if m := squashedSuffix.FindStringSubmatch(c.Subject); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				return prByNumber[n]
			}
		}
		return nil
	}

	var breaking, features, fixes, improvements []string
	releasingPR := map[int]bool{}
	for _, c := range in.Commits {
		if !c.Releasing() {
			continue
		}
		pr := prFor(c)
		var link string
		if pr != nil {
			releasingPR[pr.Number] = true
			link = fmt.Sprintf("[#%d](%s)", pr.Number, pr.URL)
		} else {
			link = fmt.Sprintf("[%s](%s/commit/%s)", shortHash(c.Hash), repo, c.Hash)
		}
		entry := fmt.Sprintf("- %s (%s)", c.Text(), link)
		switch {
		case c.Breaking:
			breaking = append(breaking, entry)
		case c.Type == "feat":
			features = append(features, entry)
		case c.Type == "fix":
			fixes = append(fixes, entry)
		default:
			improvements = append(improvements, entry)
		}
	}

	var other []string
	issues := map[string]Issue{}
	for _, pr := range in.PRs {
		if !releasingPR[pr.Number] {
			other = append(other, fmt.Sprintf("- %s ([#%d](%s))", pr.Title, pr.Number, pr.URL))
		}
		for _, issue := range pr.Closes {
			key := issue.URL
			if key == "" {
				key = strconv.Itoa(issue.Number)
			}
			issues[key] = issue
		}
	}
	closed := make([]Issue, 0, len(issues))
	for _, issue := range issues {
		closed = append(closed, issue)
	}
	sort.Slice(closed, func(i, j int) bool {
		if closed[i].Number != closed[j].Number {
			return closed[i].Number < closed[j].Number
		}
		return closed[i].URL < closed[j].URL
	})
	var closedLines []string
	for _, issue := range closed {
		closedLines = append(closedLines, fmt.Sprintf("- [#%d](%s) %s", issue.Number, issue.URL, issue.Title))
	}

	var b strings.Builder
	section := func(title string, lines []string) {
		if len(lines) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString("## " + title + "\n\n")
		b.WriteString(strings.Join(lines, "\n") + "\n")
	}
	section("Breaking changes", breaking)
	section("Features", features)
	section("Fixes", fixes)
	section("Improvements", improvements)
	section("Other pull requests", other)
	section("Closed issues", closedLines)

	if b.Len() == 0 {
		b.WriteString("No user-facing changes.\n")
	}
	if in.Previous != "" {
		fmt.Fprintf(&b, "\n**Full changelog**: [%s...%s](%s/compare/%s...%s)\n",
			in.Previous, in.Version, repo, in.Previous, in.Version)
	}
	return b.String()
}
