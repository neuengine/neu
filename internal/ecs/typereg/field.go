package typereg

import (
	"reflect"
	"strconv"
	"strings"
)

// StorageStrategy mirrors the storage-strategy enum used by the component
// registry. It exists in this package so type-level metadata can carry a
// declared default before a type is necessarily registered as a component.
type StorageStrategy uint8

const (
	// StorageTable is the default dense column-oriented layout.
	StorageTable StorageStrategy = iota
	// StorageSparseSet selects entity-indexed sparse storage.
	StorageSparseSet
)

// String returns the canonical lower-case name used in `ecs:"storage:..."`
// struct tags.
func (s StorageStrategy) String() string {
	switch s {
	case StorageSparseSet:
		return "sparse"
	default:
		return "table"
	}
}

// TypeTags collects type-level attributes parsed from a meta `_` field's
// struct tag. Example:
//
//	type Health struct {
//	    _   struct{} `ecs:"storage:sparse"`
//	    Now int
//	}
//
// Yields TypeTags{Storage: StorageSparseSet}.
type TypeTags struct {
	Storage StorageStrategy
}

// FieldTags holds parsed struct-tag metadata for a single field. Recognised
// namespaces are `ecs:"..."`, `editor:"..."`, and `range:"min,max"`. Any
// unparsed tag string is preserved verbatim in [FieldTags.Raw].
type FieldTags struct {
	Label    string
	Raw      reflect.StructTag
	RangeMax float64
	RangeMin float64
	Storage  StorageStrategy
	HasRange bool
	Hidden   bool
	Ignore   bool
	ReadOnly bool
}


// FieldInfo stores cached metadata for a single struct field. Offsets and
// types are populated once at registration so hot-path access avoids reflect
// scans of the parent type.
type FieldInfo struct {
	Type     reflect.Type // field's reflect.Type
	Name     string       // Go field name
	Tags     FieldTags    // parsed struct tag attributes
	Offset   uintptr      // byte offset within the parent struct
	Index    int          // field's zero-based index in the parent struct
	TypeID   TypeID       // late-bound TypeID (0 if the field's type is not registered)
	Exported bool         // whether the field is exported (Go's PkgPath rule)
}

// extractFields walks t's fields (when t is a struct) and produces cached
// metadata for each. The synthetic `_` meta-field used to declare type-level
// tags is intentionally skipped from the returned slice — callers that need
// type-level tags use [extractTypeTags].
func extractFields(t reflect.Type) []FieldInfo {
	if t.Kind() != reflect.Struct {
		return nil
	}
	n := t.NumField()
	if n == 0 {
		return nil
	}
	out := make([]FieldInfo, 0, n)
	for i := range n {
		f := t.Field(i)
		if f.Name == "_" {
			continue
		}
		out = append(out, FieldInfo{
			Name:     f.Name,
			Type:     f.Type,
			Offset:   f.Offset,
			Index:    i,
			Tags:     parseFieldTags(f.Tag),
			Exported: f.IsExported(),
		})
	}
	return out
}

// extractTypeTags reads the synthetic `_` meta-field on t and returns the
// type-level [TypeTags] it declares. Non-struct types and structs without a
// `_` meta-field yield zero-value tags (storage defaults to [StorageTable]).
func extractTypeTags(t reflect.Type) TypeTags {
	if t.Kind() != reflect.Struct {
		return TypeTags{}
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Name != "_" || f.Tag == "" {
			continue
		}
		ft := parseFieldTags(f.Tag)
		return TypeTags{Storage: ft.Storage}
	}
	return TypeTags{}
}

// parseFieldTags extracts every recognised attribute from tag. Each
// namespace is parsed independently; unrecognised entries within a namespace
// are silently ignored (zero-valued attributes signal absence).
func parseFieldTags(tag reflect.StructTag) FieldTags {
	ft := FieldTags{Raw: tag}

	if v, ok := tag.Lookup("ecs"); ok {
		parseECSNamespace(v, &ft)
	}
	if v, ok := tag.Lookup("editor"); ok {
		parseEditorNamespace(v, &ft)
	}
	if v, ok := tag.Lookup("range"); ok {
		parseRangeNamespace(v, &ft)
	}
	return ft
}

func parseECSNamespace(s string, ft *FieldTags) {
	for _, part := range splitTrimmed(s, ',') {
		switch {
		case part == "ignore":
			ft.Ignore = true
		case strings.HasPrefix(part, "storage:"):
			switch strings.TrimPrefix(part, "storage:") {
			case "table":
				ft.Storage = StorageTable
			case "sparse", "sparseset", "sparse_set":
				ft.Storage = StorageSparseSet
			}
		}
	}
}

func parseEditorNamespace(s string, ft *FieldTags) {
	for _, part := range splitTrimmed(s, ',') {
		switch {
		case part == "hidden":
			ft.Hidden = true
		case part == "readonly":
			ft.ReadOnly = true
		case strings.HasPrefix(part, "label:"):
			ft.Label = strings.TrimPrefix(part, "label:")
		}
	}
}

func parseRangeNamespace(s string, ft *FieldTags) {
	parts := splitTrimmed(s, ',')
	if len(parts) != 2 {
		return
	}
	minV, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return
	}
	maxV, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return
	}
	ft.HasRange = true
	ft.RangeMin = minV
	ft.RangeMax = maxV
}

// splitTrimmed splits s by sep and trims whitespace from each fragment.
// Empty fragments (from leading, trailing, or consecutive separators) are
// dropped so the result contains only meaningful parts.
func splitTrimmed(s string, sep byte) []string {
	if s == "" {
		return nil
	}
	out := make([]string, 0, 4)
	start := 0
	for i := range len(s) {
		if s[i] == sep {
			if frag := strings.TrimSpace(s[start:i]); frag != "" {
				out = append(out, frag)
			}
			start = i + 1
		}
	}
	if frag := strings.TrimSpace(s[start:]); frag != "" {
		out = append(out, frag)
	}
	return out
}
