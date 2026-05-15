package world

import (
	"reflect"
	"sync"
)

// ResourceMap stores global singleton resources keyed by Go type.
// Values are stored as pointers (*T as any) so callers can obtain stable
// mutable references via Resource[T] and SetResource[T].
// Read methods (get, contains, Len) are safe for concurrent use.
type ResourceMap struct {
	store map[reflect.Type]any
	mu    sync.RWMutex
}

// NewResourceMap returns an empty ResourceMap.
func NewResourceMap() *ResourceMap {
	return &ResourceMap{store: make(map[reflect.Type]any)}
}

func (m *ResourceMap) set(t reflect.Type, v any) {
	m.mu.Lock()
	m.store[t] = v
	m.mu.Unlock()
}

func (m *ResourceMap) get(t reflect.Type) (any, bool) {
	m.mu.RLock()
	v, ok := m.store[t]
	m.mu.RUnlock()
	return v, ok
}

func (m *ResourceMap) remove(t reflect.Type) bool {
	m.mu.Lock()
	_, ok := m.store[t]
	if ok {
		delete(m.store, t)
	}
	m.mu.Unlock()
	return ok
}

func (m *ResourceMap) contains(t reflect.Type) bool {
	m.mu.RLock()
	_, ok := m.store[t]
	m.mu.RUnlock()
	return ok
}

// Len returns the number of stored resources.
func (m *ResourceMap) Len() int {
	m.mu.RLock()
	n := len(m.store)
	m.mu.RUnlock()
	return n
}

// SetResource inserts or overwrites the singleton resource of type T.
// The value is heap-allocated so Resource[T] can return a stable pointer.
func SetResource[T any](w *World, value T) {
	t := reflect.TypeFor[T]()
	p := new(T)
	*p = value
	w.resources.set(t, p)
}

// Resource returns a read-only pointer to the singleton resource of type T.
// Returns (nil, false) if the resource has not been set.
func Resource[T any](w *World) (*T, bool) {
	t := reflect.TypeFor[T]()
	v, ok := w.resources.get(t)
	if !ok {
		return nil, false
	}
	return v.(*T), true
}

// RemoveResource removes the resource of type T and returns true if it existed.
func RemoveResource[T any](w *World) bool {
	return w.resources.remove(reflect.TypeFor[T]())
}

// ContainsResource reports whether a resource of type T exists.
func ContainsResource[T any](w *World) bool {
	return w.resources.contains(reflect.TypeFor[T]())
}
