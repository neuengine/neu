// examples/config validates the definition system end-to-end (T-6T06, C29 P6
// gate): decode a UI definition, validate it (INV-1/4), and instantiate it to a
// recording command sink (INV-2 — only commands, no world access). The hash
// over the emitted command sequence is stable across ≥20 runs.
//
// Bootstrap: validates l2-definition-system-go against l1-definition-system.
package main

import (
	"fmt"

	"github.com/neuengine/neu/pkg/definition"
)

// resolver is a TypeResolver backed by a known-name set.
type resolver struct{ known map[string]bool }

func (r resolver) ResolveType(name string) bool { return r.known[name] }

// recSink records the command sequence Instantiate emits (INV-2: no world).
type recSink struct {
	log  []string
	next definition.EntityRef
}

func (s *recSink) SpawnEntity(name string) definition.EntityRef {
	r := s.next
	s.next++
	s.log = append(s.log, "spawn:"+name)
	return r
}
func (s *recSink) InsertComponent(e definition.EntityRef, t string, _ map[string]any) {
	s.log = append(s.log, fmt.Sprintf("insert:%d:%s", e, t))
}
func (s *recSink) SetParent(c, p definition.EntityRef) {
	s.log = append(s.log, fmt.Sprintf("parent:%d<-%d", c, p))
}
func (s *recSink) RunAction(a definition.Action) { s.log = append(s.log, "action:"+a.Action) }

const menuUI = `{
  "definition":"ui","version":"1.0",
  "metadata":{"name":"main_menu"},
  "content":{"root":{"type":"Node","style":{"flex_direction":"column"},"children":[
    {"type":"Text","value":"My Game"},
    {"type":"Button","id":"play","on_click":{"action":"transition","target":"gameplay"}},
    {"type":"Button","id":"quit","on_click":{"action":"quit"}}
  ]}}}`

func run() (uint64, error) {
	types := resolver{known: map[string]bool{"Node": true, "Text": true, "Button": true}}
	def, err := definition.Decode([]byte(menuUI), types, definition.NewActionRegistry())
	if err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}
	if def.Kind != definition.KindUI {
		return 0, fmt.Errorf("kind = %v, want ui", def.Kind)
	}

	var sink recSink
	definition.Instantiate(&def, &sink) // INV-1: validated ⇒ infallible

	// INV-5 spot-check: an a→b→a include graph is rejected.
	cyclic := map[string][]string{"a.json": {"b.json"}, "b.json": {"a.json"}}
	if definition.CheckIncludeDAG(cyclic) == nil {
		return 0, fmt.Errorf("INV-5: cyclic include graph must be rejected")
	}

	h := fnvOffset
	for _, e := range sink.log {
		h = hashStr(h, e)
	}
	return h, nil
}

const (
	fnvOffset uint64 = 14695981039346656037
	fnvPrime  uint64 = 1099511628211
)

func hashStr(h uint64, s string) uint64 {
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: config (definition) hash=%d\n", h)
}
