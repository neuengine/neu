// Command releasenotes generates release documentation from the Magic SDD spec
// corpus: a dated changelog assembled from each spec's "Document History" table,
// and a breaking-change report derived from version transitions (Track E /
// T-6E03). It reuses pkg/version so "breaking" means exactly what plugin
// compatibility means — a caret-boundary crossing.
package main

import (
	"strings"
)

// HistoryEntry is one row of a spec's Document History table.
type HistoryEntry struct {
	Version string
	Date    string // YYYY-MM-DD
	Desc    string
}

// SpecDoc is the parsed metadata of a single specification file.
type SpecDoc struct {
	Path    string
	Title   string
	Version string
	Status  string
	History []HistoryEntry
}

// parseSpec extracts the title, header fields, and Document History from a spec's
// markdown content.
func parseSpec(path, content string) SpecDoc {
	doc := SpecDoc{Path: path}
	lines := strings.Split(content, "\n")
	inHistory := false
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		switch {
		case doc.Title == "" && strings.HasPrefix(line, "# "):
			doc.Title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "**Version:**"):
			doc.Version = fieldValue(line)
		case strings.HasPrefix(line, "**Status:**"):
			doc.Status = fieldValue(line)
		case strings.HasPrefix(line, "## ") && strings.Contains(line, "Document History"):
			inHistory = true
		case inHistory && strings.HasPrefix(line, "## "):
			inHistory = false // next section ends the history block
		case inHistory && strings.HasPrefix(line, "|"):
			if e, ok := parseHistoryRow(line); ok {
				doc.History = append(doc.History, e)
			}
		}
	}
	// Fall back to the file path when a spec has no parseable H1 title, so every
	// spec is still identifiable in the report.
	if doc.Title == "" {
		doc.Title = path
	}
	return doc
}

// fieldValue returns the text after a "**Field:**" prefix.
func fieldValue(line string) string {
	if _, after, ok := strings.Cut(line, ":**"); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// parseHistoryRow parses one markdown table row into a HistoryEntry. It rejects
// the header row, the "| :--- |" separator, and placeholder rows whose date is
// not a real YYYY-MM-DD date (e.g. "| — | — | Planned… |").
func parseHistoryRow(line string) (HistoryEntry, bool) {
	cells := splitRow(line)
	if len(cells) < 3 {
		return HistoryEntry{}, false
	}
	ver, date := cells[0], cells[1]
	desc := strings.Join(cells[2:], " | ")
	// Skip header + separator rows.
	if strings.EqualFold(ver, "Version") || strings.HasPrefix(ver, ":--") || strings.HasPrefix(ver, "---") {
		return HistoryEntry{}, false
	}
	if !isISODate(date) {
		return HistoryEntry{}, false // placeholder / non-dated row
	}
	return HistoryEntry{Version: ver, Date: date, Desc: desc}, true
}

// splitRow splits a markdown table row into trimmed cell values, dropping the
// empty leading/trailing fields produced by the outer pipes.
func splitRow(line string) []string {
	parts := strings.Split(line, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	// Drop leading/trailing empties from the bounding pipes.
	for len(out) > 0 && out[0] == "" {
		out = out[1:]
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}

// isISODate reports whether s is a YYYY-MM-DD date.
func isISODate(s string) bool {
	if len(s) != 10 || s[4] != '-' || s[7] != '-' {
		return false
	}
	for i, c := range s {
		if i == 4 || i == 7 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
