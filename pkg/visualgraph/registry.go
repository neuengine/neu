package visualgraph

import (
	"sort"
	"strings"
	"sync"
)

// PinDescriptor is the static definition of a node type's pin.
type PinDescriptor struct {
	ID        string
	Name      string
	DataType  string
	Direction Direction
	Kind      Kind
	Optional  bool
}

// NodeDescriptor describes a registered node type — the source of truth the
// editor queries to populate its palette (L1 §4.5).
type NodeDescriptor struct {
	TypeName    string // unique, e.g. "math.Add"
	DisplayName string
	Category    string // Event | Action | Query | Data | Flow | Variable | Component | SubGraph
	Description string
	Pins        []PinDescriptor
	DynamicPins bool
}

// NodeRegistry holds all available node types. It is read-mostly: populated at
// startup (built-ins + auto-generated from the TypeRegistry), then queried by
// the editor and the interpreter.
type NodeRegistry struct {
	types map[string]NodeDescriptor
	mu    sync.RWMutex
}

// NewNodeRegistry returns an empty registry.
func NewNodeRegistry() *NodeRegistry {
	return &NodeRegistry{types: make(map[string]NodeDescriptor)}
}

// Register adds or replaces a node type descriptor.
func (r *NodeRegistry) Register(d NodeDescriptor) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.types[d.TypeName] = d
}

// Unregister removes a node type.
func (r *NodeRegistry) Unregister(typeName string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.types, typeName)
}

// Get returns a descriptor by type name.
func (r *NodeRegistry) Get(typeName string) (NodeDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.types[typeName]
	return d, ok
}

// List returns all descriptors sorted by type name (deterministic).
func (r *NodeRegistry) List() []NodeDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]NodeDescriptor, 0, len(r.types))
	for _, d := range r.types {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TypeName < out[j].TypeName })
	return out
}

// ListByCategory returns descriptors in a category, sorted by type name.
func (r *NodeRegistry) ListByCategory(category string) []NodeDescriptor {
	var out []NodeDescriptor
	for _, d := range r.List() {
		if d.Category == category {
			out = append(out, d)
		}
	}
	return out
}

// Search returns descriptors whose type or display name contains query
// (case-insensitive), sorted by type name.
func (r *NodeRegistry) Search(query string) []NodeDescriptor {
	q := strings.ToLower(query)
	var out []NodeDescriptor
	for _, d := range r.List() {
		if strings.Contains(strings.ToLower(d.TypeName), q) ||
			strings.Contains(strings.ToLower(d.DisplayName), q) {
			out = append(out, d)
		}
	}
	return out
}

// Len returns the number of registered node types.
func (r *NodeRegistry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.types)
}
