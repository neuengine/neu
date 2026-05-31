package main

import (
	"bytes"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenSourceIsValidGo(t *testing.T) {
	t.Parallel()
	src, err := genSource(4, 6)
	if err != nil {
		t.Fatalf("genSource: %v", err)
	}
	// format.Source already validated syntax; re-parse to be explicit.
	if _, err := parser.ParseFile(token.NewFileSet(), "query_gen.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v", err)
	}
	text := string(src)
	for _, want := range []string{
		"DO NOT EDIT",
		"package query",
		"type Tuple4[A, B, C, D any] struct",
		"type Query5[A, B, C, D, E any] struct",
		"func NewQuery6[A, B, C, D, E, F any]",
		"[6]component.ID",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated source missing %q", want)
		}
	}
}

func TestGenArityShape(t *testing.T) {
	t.Parallel()
	a4 := genArity(4)
	// Tuple has exactly the four pointer fields.
	for _, f := range []string{"A *A", "B *B", "C *C", "D *D"} {
		if !strings.Contains(a4, f) {
			t.Errorf("Tuple4 missing field %q", f)
		}
	}
	// NewQuery4 resolves four component IDs and checks all distinct pairs.
	if strings.Count(a4, "componentIDFor[") != 4 {
		t.Errorf("NewQuery4 should resolve 4 component IDs:\n%s", a4)
	}
	if !strings.Contains(a4, "idC == idD") {
		t.Error("NewQuery4 missing the idC==idD distinctness pair")
	}
	// All fetches each component by its ids index.
	if !strings.Contains(a4, "q.ids[3]") {
		t.Error("Query4.All should fetch component at ids[3]")
	}
}

func TestLetterAndDistinctCond(t *testing.T) {
	t.Parallel()
	if letter(0) != "A" || letter(2) != "C" || letter(5) != "F" {
		t.Errorf("letter mapping wrong: %q %q %q", letter(0), letter(2), letter(5))
	}
	got := distinctCond([]string{"idA", "idB", "idC"})
	want := "idA == idB || idA == idC || idB == idC"
	if got != want {
		t.Errorf("distinctCond = %q, want %q", got, want)
	}
}

func TestRunCLI(t *testing.T) {
	t.Parallel()
	// stdout mode.
	var out, errb bytes.Buffer
	if code := run([]string{"-min", "4", "-max", "5"}, &out, &errb); code != 0 {
		t.Fatalf("run exit = %d, err=%s", code, errb.String())
	}
	if !strings.Contains(out.String(), "type Query4") || !strings.Contains(out.String(), "type Query5") {
		t.Errorf("stdout missing arities:\n%s", out.String())
	}

	// file mode.
	outFile := filepath.Join(t.TempDir(), "gen.go")
	out.Reset()
	if code := run([]string{"-min", "4", "-max", "4", "-out", outFile}, &out, &errb); code != 0 {
		t.Fatalf("file-mode exit = %d", code)
	}
	data, err := os.ReadFile(outFile)
	if err != nil || !strings.Contains(string(data), "type Query4") {
		t.Fatalf("output file wrong: err=%v", err)
	}

	// invalid ranges and bad flags ⇒ exit 2.
	for _, argv := range [][]string{
		{"-min", "1", "-max", "4"},  // min < 2
		{"-min", "6", "-max", "4"},  // max < min
		{"-min", "4", "-max", "99"}, // max > 26
		{"-bogus"},
	} {
		out.Reset()
		errb.Reset()
		if code := run(argv, &out, &errb); code != 2 {
			t.Errorf("run(%v) exit = %d, want 2", argv, code)
		}
	}
}
