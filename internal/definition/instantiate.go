// Package definition realizes validated pkg/definition assets into a World: it
// adapts the engine-agnostic definition.CommandSink to real ECS commands
// (spawn, component insert via the type registry, ChildOf parenting) and tracks
// the entities each definition produced so a hot-reload can despawn and
// re-instantiate them (the consuming hot-reload system is T-6A03 proper; this is
// its instantiation + instance-registry foundation).
package definition

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/world"
	def "github.com/neuengine/neu/pkg/definition"
)

// TypeResolver maps a definition's component type name to its Go type. The app
// wires it to its type registry (e.g. typereg.ResolveByName → reg.Type); tests
// inject a map. It is the realization-time counterpart to the validation-time
// definition.TypeResolver (which only answers "does this name exist?").
type TypeResolver func(name string) (reflect.Type, bool)

// worldSink realizes definition commands against a World. EntityRefs are indices
// into refs (the order definition.Instantiate spawns them); components are built
// from their decoded value maps and inserted (auto-registering the type). Boundary
// mismatches — a name the resolver doesn't know, or a value that won't decode into
// its Go type — are collected in errs rather than silently dropped.
type worldSink struct {
	w       *world.World
	resolve TypeResolver
	refs    []entity.Entity
	actions []def.Action
	errs    []error
}

func (s *worldSink) SpawnEntity(_ string) def.EntityRef {
	ref := def.EntityRef(len(s.refs))
	s.refs = append(s.refs, s.w.SpawnEmpty())
	return ref
}

func (s *worldSink) InsertComponent(ref def.EntityRef, typeName string, value map[string]any) {
	e, ok := s.entityFor(ref)
	if !ok {
		return
	}
	t, ok := s.resolve(typeName)
	if !ok {
		s.errs = append(s.errs, fmt.Errorf("definition: no Go type registered for component %q", typeName))
		return
	}
	val, err := construct(t, value)
	if err != nil {
		s.errs = append(s.errs, fmt.Errorf("definition: build %q: %w", typeName, err))
		return
	}
	// Insert without an explicit ID auto-registers the component type by value.
	_ = s.w.Insert(e, component.Data{Value: val})
}

func (s *worldSink) SetParent(child, parent def.EntityRef) {
	c, ok1 := s.entityFor(child)
	p, ok2 := s.entityFor(parent)
	if !ok1 || !ok2 {
		return
	}
	_ = s.w.Insert(c, component.Data{Value: hierarchy.ChildOf{Parent: p}})
}

// RunAction defers declarative actions: they are collected on the Instance for a
// later state-machine / action-dispatch pass (Flow integration), not executed
// here — instantiation only produces entities.
func (s *worldSink) RunAction(a def.Action) { s.actions = append(s.actions, a) }

func (s *worldSink) entityFor(ref def.EntityRef) (entity.Entity, bool) {
	if ref < 0 || int(ref) >= len(s.refs) {
		s.errs = append(s.errs, fmt.Errorf("definition: command referenced unknown entity ref %d", ref))
		return entity.Entity{}, false
	}
	return s.refs[ref], true
}

// construct builds a value of type t from a decoded JSON value map by round-
// tripping through encoding/json — reusing the stdlib decoder's coercion
// (number widening, nested structs, slices) that a field-by-field set cannot do.
func construct(t reflect.Type, value map[string]any) (any, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	ptr := reflect.New(t)
	if err := json.Unmarshal(b, ptr.Interface()); err != nil {
		return nil, err
	}
	return ptr.Elem().Interface(), nil
}

// Instance is the set of entities (and deferred actions) one definition produced,
// so a hot-reload can despawn and re-instantiate it as a unit.
type Instance struct {
	Entities []entity.Entity
	Actions  []def.Action
}

// Instantiate realizes d into w, resolving component type names via resolve, and
// returns the spawned Instance plus any boundary errors. d is assumed validated
// (definition INV-1); the returned errors are adapter-boundary mismatches only
// (a type the app forgot to register, or a value that won't decode).
func Instantiate(w *world.World, d *def.Definition, resolve TypeResolver) (Instance, []error) {
	s := &worldSink{w: w, resolve: resolve}
	def.Instantiate(d, s)
	return Instance{Entities: s.refs, Actions: s.actions}, s.errs
}

// Despawn removes every entity in the instance from w. Safe to call twice; an
// already-dead entity is a no-op.
func (in Instance) Despawn(w *world.World) {
	for _, e := range in.Entities {
		_ = w.Despawn(e)
	}
}

// InstanceStore tracks the live Instance produced by each definition path, so a
// reload despawns the prior entities before spawning the new ones. Stored as a
// World resource; not safe for concurrent use (driven by the schedule executor).
type InstanceStore struct {
	byPath map[string]Instance
}

// NewInstanceStore returns an empty store.
func NewInstanceStore() *InstanceStore {
	return &InstanceStore{byPath: make(map[string]Instance)}
}

// Get returns the live instance for path, if any.
func (s *InstanceStore) Get(path string) (Instance, bool) {
	in, ok := s.byPath[path]
	return in, ok
}

// Instantiate spawns d for path and records it as the path's live instance. A
// prior instance for the same path is NOT despawned — use [InstanceStore.Reload]
// for hot-reload semantics.
func (s *InstanceStore) Instantiate(w *world.World, path string, d *def.Definition, resolve TypeResolver) (Instance, []error) {
	in, errs := Instantiate(w, d, resolve)
	s.byPath[path] = in
	return in, errs
}

// Reload despawns path's prior instance (if any) and instantiates d in its place
// — the hot-reload primitive a definition AssetEvent[Modified] handler calls.
func (s *InstanceStore) Reload(w *world.World, path string, d *def.Definition, resolve TypeResolver) (Instance, []error) {
	if old, ok := s.byPath[path]; ok {
		old.Despawn(w)
	}
	return s.Instantiate(w, path, d, resolve)
}
