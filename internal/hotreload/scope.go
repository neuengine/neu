//go:build editor

package hotreload

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
)

// ChangeScope classifies how a Go source change affects a running World, which
// the orchestrator uses to decide whether a full state restore is valid
// (l1-hot-reload §4.10).
type ChangeScope uint8

const (
	// ScopeSystemOnly: only function bodies changed → the World snapshot is
	// fully valid (the fast, common path: tweaking system logic). Also the
	// fallback when classification is uncertain — restore validates each
	// component anyway, so an optimistic SystemOnly degrades to per-component
	// drops, never corruption.
	ScopeSystemOnly ChangeScope = iota
	// ScopeComponentType: a top-level type's shape changed (added/removed/edited
	// struct fields, or a type was added/removed) → affected components may not
	// deserialize and are dropped on restore.
	ScopeComponentType
)

// String returns the scope name.
func (s ChangeScope) String() string {
	if s == ScopeComponentType {
		return "ComponentType"
	}
	return "SystemOnly"
}

// ClassifyChange compares two versions of a Go source file and reports the
// change scope. It is best-effort (l1-hot-reload §4.10): if only function bodies
// differ it returns ScopeSystemOnly; if any top-level type's structure changed
// (or a type was added/removed) it returns ScopeComponentType; a parse failure
// falls back to ScopeSystemOnly (the safest common case, since restore validates
// each component regardless).
func ClassifyChange(oldSrc, newSrc []byte) ChangeScope {
	oldShapes, err1 := typeShapes(oldSrc)
	newShapes, err2 := typeShapes(newSrc)
	if err1 != nil || err2 != nil {
		return ScopeSystemOnly
	}
	if len(oldShapes) != len(newShapes) {
		return ScopeComponentType
	}
	for name, newShape := range newShapes {
		oldShape, ok := oldShapes[name]
		if !ok || oldShape != newShape {
			return ScopeComponentType
		}
	}
	return ScopeSystemOnly
}

// typeShapes parses src and returns a map from each top-level type name to a
// canonical rendering of its underlying type expression, so two versions can be
// compared structurally while ignoring formatting and function-body edits.
func typeShapes(src []byte) (map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "", src, 0)
	if err != nil {
		return nil, err
	}
	shapes := make(map[string]string)
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			var buf bytes.Buffer
			if err := printer.Fprint(&buf, fset, ts.Type); err != nil {
				continue
			}
			shapes[ts.Name.Name] = buf.String()
		}
	}
	return shapes, nil
}
