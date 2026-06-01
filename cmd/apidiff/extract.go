// Command apidiff snapshots the engine's exported API surface and fails on an
// undocumented breaking change (Track J / T-6J02). It is the API-stability member
// of the engine's "committed snapshot → drift" tooling family, alongside
// cmd/benchcompare (perf baselines) and cmd/examplecheck (example goldens); the
// SemVer policy in pkg/version says what "breaking" means, this detects it.
//
// Extraction is stdlib-only (go/ast + go/printer, C-003): signatures are rendered
// through go/printer so reformatting the source produces no spurious diff — only
// a real signature change does.
package main

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"sort"
	"strings"
)

// Symbol is one exported API element. Methods are qualified by their receiver
// type (e.g. "World.Spawn"); the rendered Signature is the change discriminator.
type Symbol struct {
	Name      string `json:"name"`
	Kind      string `json:"kind"` // func | method | type | const | var
	Signature string `json:"signature"`
}

// extractFromSource parses one Go source file and returns its exported symbols,
// sorted by (kind, name). Test files should be excluded by the caller.
func extractFromSource(filename, src string) ([]Symbol, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var syms []Symbol
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			sig := "func " + d.Name.Name + strings.TrimPrefix(renderNode(fset, d.Type), "func")
			if d.Recv == nil {
				syms = append(syms, Symbol{Name: d.Name.Name, Kind: "func", Signature: sig})
				continue
			}
			recv := baseTypeName(d.Recv.List[0].Type)
			if recv == "" || !ast.IsExported(recv) {
				continue // methods on unexported types are not public API
			}
			syms = append(syms, Symbol{Name: recv + "." + d.Name.Name, Kind: "method", Signature: sig})
		case *ast.GenDecl:
			syms = append(syms, genDeclSymbols(fset, d)...)
		}
	}
	sort.Slice(syms, func(i, j int) bool {
		if syms[i].Kind != syms[j].Kind {
			return syms[i].Kind < syms[j].Kind
		}
		return syms[i].Name < syms[j].Name
	})
	return syms, nil
}

// genDeclSymbols extracts exported type/const/var symbols from a GenDecl.
func genDeclSymbols(fset *token.FileSet, d *ast.GenDecl) []Symbol {
	var syms []Symbol
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if !s.Name.IsExported() {
				continue
			}
			op := " "
			if s.Assign.IsValid() {
				op = " = " // type alias
			}
			syms = append(syms, Symbol{
				Name:      s.Name.Name,
				Kind:      "type",
				Signature: "type " + s.Name.Name + op + renderNode(fset, s.Type),
			})
		case *ast.ValueSpec:
			kind := "var"
			if d.Tok == token.CONST {
				kind = "const"
			}
			for _, n := range s.Names {
				if !n.IsExported() {
					continue
				}
				sig := kind + " " + n.Name
				if s.Type != nil {
					sig += " " + renderNode(fset, s.Type)
				}
				syms = append(syms, Symbol{Name: n.Name, Kind: kind, Signature: sig})
			}
		}
	}
	return syms
}

// renderNode renders an AST node to its canonical go/printer form.
func renderNode(fset *token.FileSet, node ast.Node) string {
	var buf strings.Builder
	if err := printer.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	return strings.Join(strings.Fields(buf.String()), " ") // normalize whitespace
}

// baseTypeName unwraps a receiver expression to its base type identifier,
// handling pointers and generic receivers (T, *T, T[A], T[A,B]).
func baseTypeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return baseTypeName(e.X)
	case *ast.IndexExpr:
		return baseTypeName(e.X)
	case *ast.IndexListExpr:
		return baseTypeName(e.X)
	}
	return ""
}
