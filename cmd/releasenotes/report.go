package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/neuengine/neu/pkg/version"
)

// ReleaseNotes renders a markdown changelog of every Document History entry dated
// on or after since (a YYYY-MM-DD cutoff; "" includes all entries), grouped by
// spec title. Specs with no qualifying entries are omitted.
func ReleaseNotes(specs []SpecDoc, since string) string {
	sorted := append([]SpecDoc(nil), specs...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Title < sorted[j].Title })

	var b strings.Builder
	b.WriteString("# Release Notes\n")
	if since != "" {
		fmt.Fprintf(&b, "\n_Changes since %s._\n", since)
	}
	wrote := false
	for _, s := range sorted {
		var entries []HistoryEntry
		for _, e := range s.History {
			if since == "" || e.Date >= since { // ISO dates compare lexically
				entries = append(entries, e)
			}
		}
		if len(entries) == 0 {
			continue
		}
		wrote = true
		fmt.Fprintf(&b, "\n## %s (v%s, %s)\n\n", s.Title, s.Version, s.Status)
		// Newest first within a spec.
		sort.Slice(entries, func(i, j int) bool { return entries[i].Date > entries[j].Date })
		for _, e := range entries {
			fmt.Fprintf(&b, "- **%s** (%s) — %s\n", e.Version, e.Date, e.Desc)
		}
	}
	if !wrote {
		b.WriteString("\n_No changes in range._\n")
	}
	return b.String()
}

// Breaking describes a spec whose latest version crossed the caret-compatibility
// boundary of its previous version (a breaking change under the engine policy).
type Breaking struct {
	Title    string
	Previous string
	Latest   string
}

// BreakingChanges reports specs whose two most-recent valid SemVer history
// entries are caret-incompatible. It dogfoods pkg/version: a transition is
// breaking exactly when `^previous` does not match `latest` — the same rule that
// gates plugin engine_version compatibility (so a 0.x minor bump counts).
func BreakingChanges(specs []SpecDoc) []Breaking {
	var out []Breaking
	for _, s := range specs {
		versions := validVersions(s.History)
		if len(versions) < 2 {
			continue
		}
		prev := versions[len(versions)-2]
		latest := versions[len(versions)-1]
		c, err := version.ParseConstraint("^" + prev.String())
		if err != nil {
			continue
		}
		if !c.Matches(latest) {
			out = append(out, Breaking{Title: s.Title, Previous: prev.String(), Latest: latest.String()})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

// validVersions extracts the parseable versions from a history, sorted ascending.
func validVersions(history []HistoryEntry) []version.Version {
	var vs []version.Version
	for _, e := range history {
		if v, err := version.Parse(e.Version); err == nil {
			vs = append(vs, v)
		}
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].Compare(vs[j]) < 0 })
	return vs
}

// RenderBreaking formats a breaking-change report as markdown.
func RenderBreaking(breaks []Breaking) string {
	var b strings.Builder
	b.WriteString("# Breaking Changes\n\n")
	if len(breaks) == 0 {
		b.WriteString("_None detected._\n")
		return b.String()
	}
	for _, br := range breaks {
		fmt.Fprintf(&b, "- **%s**: %s → %s (caret-incompatible)\n", br.Title, br.Previous, br.Latest)
	}
	return b.String()
}
