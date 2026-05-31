package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleSpec = `# Asset Formats

**Version:** 0.2.0
**Status:** Stable
**Layer:** concept

## Overview

Some prose.

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-03-25 | Initial draft from architecture analysis |
| 0.2.0 | 2026-05-31 | Narrowed to match implementation + promoted to Stable |
| — | — | Planned examples: examples/asset/ |
`

func TestParseSpec(t *testing.T) {
	t.Parallel()
	doc := parseSpec("l1-asset-formats.md", sampleSpec)
	if doc.Title != "Asset Formats" {
		t.Errorf("Title = %q", doc.Title)
	}
	if doc.Version != "0.2.0" || doc.Status != "Stable" {
		t.Errorf("Version/Status = %q/%q", doc.Version, doc.Status)
	}
	// Header + separator + em-dash placeholder rows are all skipped.
	if len(doc.History) != 2 {
		t.Fatalf("History = %d entries, want 2: %+v", len(doc.History), doc.History)
	}
	if doc.History[0].Version != "0.1.0" || doc.History[0].Date != "2026-03-25" {
		t.Errorf("entry[0] = %+v", doc.History[0])
	}
	if !strings.Contains(doc.History[1].Desc, "Narrowed") {
		t.Errorf("entry[1] desc = %q", doc.History[1].Desc)
	}
}

func TestParseSpecDescWithPipes(t *testing.T) {
	t.Parallel()
	// A description containing a pipe must survive cell-splitting.
	spec := "# X\n\n**Version:** 1.0.0\n**Status:** Stable\n\n## Document History\n\n" +
		"| Version | Date | Description |\n| :--- | :--- | :--- |\n" +
		"| 1.0.0 | 2026-05-31 | adds `a | b` union support |\n"
	doc := parseSpec("x.md", spec)
	if len(doc.History) != 1 {
		t.Fatalf("want 1 entry, got %d", len(doc.History))
	}
	if !strings.Contains(doc.History[0].Desc, "a | b") {
		t.Errorf("desc lost pipe: %q", doc.History[0].Desc)
	}
}

func TestReleaseNotesSinceFilter(t *testing.T) {
	t.Parallel()
	specs := []SpecDoc{
		{Title: "Beta", Version: "1.0.0", Status: "Stable", History: []HistoryEntry{
			{"0.9.0", "2026-01-01", "old"}, {"1.0.0", "2026-05-31", "new beta"},
		}},
		{Title: "Alpha", Version: "0.2.0", Status: "Stable", History: []HistoryEntry{
			{"0.2.0", "2026-05-31", "new alpha"},
		}},
		{Title: "Stale", Version: "1.0.0", Status: "Stable", History: []HistoryEntry{
			{"1.0.0", "2025-01-01", "ancient"},
		}},
	}
	out := ReleaseNotes(specs, "2026-05-01")
	// Alpha sorts before Beta; Stale (pre-cutoff) is omitted.
	ai := strings.Index(out, "## Alpha")
	bi := strings.Index(out, "## Beta")
	if ai < 0 || bi < 0 || ai > bi {
		t.Errorf("expected Alpha before Beta; out=\n%s", out)
	}
	if strings.Contains(out, "Stale") || strings.Contains(out, "ancient") || strings.Contains(out, "old") {
		t.Errorf("pre-cutoff entries leaked:\n%s", out)
	}
	if !strings.Contains(out, "new alpha") || !strings.Contains(out, "new beta") {
		t.Errorf("missing in-range entries:\n%s", out)
	}
}

func TestReleaseNotesEmptyRange(t *testing.T) {
	t.Parallel()
	specs := []SpecDoc{{Title: "A", History: []HistoryEntry{{"1.0.0", "2025-01-01", "old"}}}}
	out := ReleaseNotes(specs, "2030-01-01")
	if !strings.Contains(out, "No changes in range") {
		t.Errorf("expected empty-range note, got:\n%s", out)
	}
}

func TestBreakingChanges(t *testing.T) {
	t.Parallel()
	specs := []SpecDoc{
		// 0.x minor bump → breaking (caret on 0.x pins the minor).
		{Title: "ZeroX", History: []HistoryEntry{{"0.1.0", "2026-01-01", ""}, {"0.2.0", "2026-05-31", ""}}},
		// 1.x minor bump → NOT breaking.
		{Title: "MinorBump", History: []HistoryEntry{{"1.2.0", "2026-01-01", ""}, {"1.3.0", "2026-05-31", ""}}},
		// major bump → breaking.
		{Title: "MajorBump", History: []HistoryEntry{{"1.0.0", "2026-01-01", ""}, {"2.0.0", "2026-05-31", ""}}},
		// single version → skipped.
		{Title: "Fresh", History: []HistoryEntry{{"0.1.0", "2026-05-31", ""}}},
	}
	breaks := BreakingChanges(specs)
	got := map[string]string{}
	for _, b := range breaks {
		got[b.Title] = b.Previous + "→" + b.Latest
	}
	if _, ok := got["MinorBump"]; ok {
		t.Error("1.2.0→1.3.0 must not be breaking")
	}
	if _, ok := got["Fresh"]; ok {
		t.Error("single-version spec must be skipped")
	}
	if got["ZeroX"] != "0.1.0→0.2.0" {
		t.Errorf("ZeroX should be breaking (0.x minor), got %v", got)
	}
	if got["MajorBump"] != "1.0.0→2.0.0" {
		t.Errorf("MajorBump should be breaking, got %v", got)
	}
	// Render is non-empty and lists the breaking specs.
	r := RenderBreaking(breaks)
	if !strings.Contains(r, "ZeroX") || !strings.Contains(r, "MajorBump") {
		t.Errorf("render missing breaks:\n%s", r)
	}
	if !strings.Contains(RenderBreaking(nil), "None detected") {
		t.Error("empty breaking report should say None detected")
	}
}

func writeSpec(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunEndToEnd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "l1-asset-formats.md", sampleSpec)
	writeSpec(t, dir, "l2-other.md", "# Other\n\n**Version:** 0.2.0\n**Status:** Stable\n\n## Document History\n\n"+
		"| Version | Date | Description |\n| :--- | :--- | :--- |\n| 0.1.0 | 2026-05-01 | first |\n| 0.2.0 | 2026-05-31 | second |\n")
	writeSpec(t, dir, "notes.txt", "ignored non-markdown")

	// Release notes to stdout.
	var out, errb bytes.Buffer
	if code := run([]string{"-specs", dir, "-since", "2026-05-15"}, &out, &errb); code != 0 {
		t.Fatalf("run exit = %d, err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Release Notes") || !strings.Contains(out.String(), "Asset Formats") {
		t.Errorf("release notes output:\n%s", out.String())
	}

	// Breaking report to a file (Other 0.1.0→0.2.0 is a 0.x minor → breaking).
	outFile := filepath.Join(dir, "breaking.md")
	out.Reset()
	if code := run([]string{"-specs", dir, "-breaking", "-o", outFile}, &out, &errb); code != 0 {
		t.Fatalf("breaking run exit = %d", code)
	}
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read out file: %v", err)
	}
	if !strings.Contains(string(data), "Breaking Changes") {
		t.Errorf("breaking file:\n%s", data)
	}
}

func TestRunUsageErrors(t *testing.T) {
	t.Parallel()
	var out, errb bytes.Buffer
	// Missing specs dir → exit 2.
	if code := run([]string{"-specs", filepath.Join(t.TempDir(), "nope")}, &out, &errb); code != 2 {
		t.Errorf("missing dir exit = %d, want 2", code)
	}
	// Empty dir (no .md) → exit 2.
	if code := run([]string{"-specs", t.TempDir()}, &out, &errb); code != 2 {
		t.Errorf("empty dir exit = %d, want 2", code)
	}
	// Unknown flag → exit 2.
	if code := run([]string{"-bogus"}, &out, &errb); code != 2 {
		t.Errorf("bad flag exit = %d, want 2", code)
	}
}
