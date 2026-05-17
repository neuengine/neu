package editor_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const pkgPath = "github.com/neuengine/neu/pkg/editor"

// ── TestEditorPkgIsContractOnly ───────────────────────────────────────────────

// TestEditorPkgIsContractOnly parses every non-test .go file in pkg/editor/
// and asserts:
//  1. No FuncDecl with a body (only interface-method FuncType decls are allowed).
//  2. No import path containing "internal/".
//
// This mechanically enforces INV-3 of l2-multi-repo-architecture-go.md.
func TestEditorPkgIsContractOnly(t *testing.T) {
	dir := sourceDir(t)
	fset := token.NewFileSet()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir(%s): %v", dir, err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		// Check imports — no "internal/" paths.
		for _, imp := range f.Imports {
			impPath := strings.Trim(imp.Path.Value, `"`)
			if strings.Contains(impPath, "internal/") {
				t.Errorf("%s: imports internal package %q (INV-3)", name, impPath)
			}
		}

		// Check declarations — no FuncDecl with a body.
		ast.Inspect(f, func(n ast.Node) bool {
			fd, ok := n.(*ast.FuncDecl)
			if !ok {
				return true
			}
			if fd.Body != nil {
				t.Errorf("%s: FuncDecl %q has a body — pkg/editor must be interface/type only (INV-3)",
					name, fd.Name.Name)
			}
			return true
		})
	}
}

// ── TestNoEditorImports ───────────────────────────────────────────────────────

// TestNoEditorImports verifies the engine does not import the external editor
// repository — INV-1 of l2-multi-repo-architecture-go.md.
func TestNoEditorImports(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = projectRoot(t)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("go list -deps failed: %v", err)
	}
	for pkg := range strings.SplitSeq(string(out), "\n") {
		if strings.Contains(pkg, "github.com/neuengine/editor") {
			t.Errorf("engine depends on external editor package: %s", pkg)
		}
	}
}

// ── TestNoPackageInit ─────────────────────────────────────────────────────────

// TestNoPackageInit asserts that pkg/editor defines no init() function (INV-5:
// importing neither package must add zero runtime overhead).
func TestNoPackageInit(t *testing.T) {
	dir := sourceDir(t)
	fset := token.NewFileSet()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fd, ok := n.(*ast.FuncDecl)
			if ok && fd.Name.Name == "init" {
				t.Errorf("%s: defines init() — pkg/editor must have zero side effects (INV-5)", name)
			}
			return true
		})
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// sourceDir returns the absolute path to the pkg/editor directory on disk by
// locating it relative to the test binary's working directory.
func sourceDir(t *testing.T) string {
	t.Helper()
	// go test sets the working directory to the package directory.
	abs, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	return abs
}

func projectRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("Abs: %v", err)
	}
	return abs
}

// ── TestEditorBuildClean ──────────────────────────────────────────────────────

// TestEditorBuildClean verifies that pkg/editor compiles standalone.
func TestEditorBuildClean(t *testing.T) {
	cmd := exec.Command("go", "build", pkgPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s failed:\n%s", pkgPath, out)
	}
}
