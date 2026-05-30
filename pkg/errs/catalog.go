package errs

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"sync"
)

// catalogEntry is one localized message in a catalog file.
type catalogEntry struct {
	Template string `json:"template"`
	Solution string `json:"solution"`
}

// Catalog maps error codes to localized message templates loaded from an
// fs.FS (e.g. "errors.en.json"). A missing key degrades to the code's
// registered Descriptor.Template, then to the bare code — never empty.
type Catalog struct {
	mu        sync.RWMutex
	lang      string
	templates map[Code]string
}

// NewCatalog returns an empty catalog. Until Load succeeds it serves the
// registered default templates.
func NewCatalog() *Catalog { return &Catalog{templates: make(map[Code]string)} }

// defaultCatalog backs engineError.Error(). It is package-global so any
// EngineError renders consistently; SetDefaultCatalog swaps the active locale.
var defaultCatalog = NewCatalog()

// SetDefaultCatalog installs c as the catalog used by EngineError.Error().
func SetDefaultCatalog(c *Catalog) {
	if c != nil {
		defaultCatalog = c
	}
}

// Load reads "errors.{lang}.json" from fsys into the catalog. A missing or
// malformed file is reported as an error but leaves any previously loaded (and
// the registered default) templates intact — callers may treat the failure as
// a Warning and continue (L1 §4: never crash on a missing catalog).
func (c *Catalog) Load(fsys fs.FS, lang string) error {
	name := fmt.Sprintf("errors.%s.json", lang)
	raw, err := fs.ReadFile(fsys, name)
	if err != nil {
		return fmt.Errorf("errs: read catalog %q: %w", name, err)
	}
	var parsed map[Code]catalogEntry
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return fmt.Errorf("errs: parse catalog %q: %w", name, err)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lang = lang
	for code, entry := range parsed {
		c.templates[code] = entry.Template
	}
	return nil
}

// Localize expands the template for code with args (fmt-style verbs). Resolution
// order: catalog entry → registered Descriptor.Template → bare code. The result
// is never empty (INV fallback).
func (c *Catalog) Localize(code Code, args ...any) string {
	c.mu.RLock()
	tmpl, ok := c.templates[code]
	c.mu.RUnlock()
	if !ok {
		if d, found := Lookup(code); found && d.Template != "" {
			tmpl = d.Template
		}
	}
	if tmpl == "" {
		// Last-resort fallback: the bare code plus any args.
		if len(args) == 0 {
			return string(code)
		}
		return fmt.Sprintf("%s: %v", string(code), args)
	}
	if len(args) == 0 {
		return tmpl
	}
	return fmt.Sprintf(tmpl, args...)
}

// Lang returns the catalog's currently loaded language tag ("" if none).
func (c *Catalog) Lang() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lang
}
