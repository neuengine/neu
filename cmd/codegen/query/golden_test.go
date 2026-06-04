package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// T-6T04 codegen golden drift-gate: the committed generated artifact
// (internal/ecs/query/query_gen.go) must be byte-identical to what the generator
// produces today, so a hand-edit of the generated file or a generator change
// without regeneration is caught in CI — the codegen analog of the apidiff
// committed snapshot and the examplecheck goldens. The //go:generate directive
// uses `-min 4 -max 6`, so the golden regenerates with the same range.

// committedGenPath is the generated file, relative to this package directory
// (cmd/codegen/query → repo root is ../../..).
const committedGenPath = "../../../internal/ecs/query/query_gen.go"

// normalizeEOL strips CR so the comparison is line-ending agnostic: a Windows
// checkout may store CRLF while the generator emits LF; git normalizes to LF on
// commit and gofmt content is canonical, so only the content matters.
func normalizeEOL(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }

func TestGeneratedFileMatchesGenerator(t *testing.T) {
	t.Parallel()
	src, err := genSource(4, 6)
	if err != nil {
		t.Fatalf("genSource(4, 6): %v", err)
	}
	committed, err := os.ReadFile(filepath.Clean(committedGenPath))
	if err != nil {
		t.Fatalf("read committed %s: %v", committedGenPath, err)
	}
	if !bytes.Equal(normalizeEOL(src), normalizeEOL(committed)) {
		t.Errorf("internal/ecs/query/query_gen.go is stale — regenerate it with\n"+
			"  go generate ./internal/ecs/query\n"+
			"(the generator output differs from the committed file; %d vs %d bytes)",
			len(src), len(committed))
	}
}

func TestGenSourceDeterministic(t *testing.T) {
	t.Parallel()
	first, err := genSource(4, 6)
	if err != nil {
		t.Fatalf("genSource: %v", err)
	}
	for i := range 5 {
		again, err := genSource(4, 6)
		if err != nil {
			t.Fatalf("genSource run %d: %v", i, err)
		}
		if !bytes.Equal(first, again) {
			t.Fatalf("genSource is non-deterministic (run %d differs) — output must be stable for the golden gate", i)
		}
	}
}
