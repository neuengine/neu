package scene

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
)

// DynamicScene is an immutable snapshot of a subset of a World's entities and
// their components. It is reflection-based (portable) in contrast to the opaque
// gob blob used by StaticScene.
//
// Created by DynamicSceneBuilder.Build; the entity slice is frozen after build
// so a single DynamicScene handle can spawn N independent instances (INV-4).
type DynamicScene struct {
	entities []DynamicEntity
}

// Entities returns the captured entity snapshots (read-only view).
func (s *DynamicScene) Entities() []DynamicEntity { return s.entities }

// DynamicEntity is the per-entity snapshot: the original EntityID and a list
// of deep-copied component values together with their type metadata.
type DynamicEntity struct {
	Components []ReflectedComponent
	ID         entity.EntityID
}

// ReflectedComponent holds one component snapshot for a DynamicEntity.
// Value is a deep copy of the component data at Build time; TypeName is the
// fully-qualified name from the TypeRegistry (used as the interning key).
type ReflectedComponent struct {
	Reg   *typereg.TypeRegistration
	Value reflect.Value // deep copy — not connected to the source World
}

// SceneFilter controls which component types are included in a DynamicScene
// extraction. A type is included when it is in the allow-set (or when the
// allow-set is empty, meaning all types), unless it is also in the deny-set.
// Deny always wins (INV-3).
type SceneFilter struct {
	allow map[string]struct{}
	deny  map[string]struct{}
}

// NewSceneFilter creates a filter. allowNames and denyNames are fully-qualified
// type names from the TypeRegistry. Empty allowNames means "allow all".
func NewSceneFilter(allowNames, denyNames []string) SceneFilter {
	f := SceneFilter{
		allow: make(map[string]struct{}, len(allowNames)),
		deny:  make(map[string]struct{}, len(denyNames)),
	}
	for _, n := range allowNames {
		f.allow[n] = struct{}{}
	}
	for _, n := range denyNames {
		f.deny[n] = struct{}{}
	}
	return f
}

// included reports whether a type name passes the filter.
func (f SceneFilter) included(typeName string) bool {
	if _, denied := f.deny[typeName]; denied {
		return false
	}
	if len(f.allow) == 0 {
		return true
	}
	_, allowed := f.allow[typeName]
	return allowed
}

// WorldReader is a minimal interface over world.World used by the builder,
// making the builder testable without a full ECS world.
type WorldReader interface {
	// EachArchetype calls fn for every archetype (return false to stop).
	EachArchetype(fn func(arch ArchetypeView) bool)
}

// ArchetypeView exposes the parts of an archetype the builder needs.
type ArchetypeView interface {
	// Entities returns the entity slice (do not mutate).
	Entities() []entity.Entity
	// ComponentValues returns a snapshot of all component values for the given
	// row index as a map from fully-qualified type name to the Go value.
	ComponentValues(row int) map[string]any
}

// DynamicSceneBuilder constructs a DynamicScene by extracting entities from a
// WorldReader using a TypeRegistry for metadata lookup.
type DynamicSceneBuilder struct {
	world  WorldReader
	reg    *typereg.TypeRegistry
	filter SceneFilter
	only   []entity.EntityID // nil = extract all
}

// NewDynamicSceneBuilder creates a builder for the given world and type registry.
func NewDynamicSceneBuilder(w WorldReader, reg *typereg.TypeRegistry) *DynamicSceneBuilder {
	return &DynamicSceneBuilder{world: w, reg: reg}
}

// WithFilter sets the component inclusion filter; returns the builder for chaining.
func (b *DynamicSceneBuilder) WithFilter(f SceneFilter) *DynamicSceneBuilder {
	b.filter = f
	return b
}

// ExtractEntity adds a specific entity to the extraction list. When at least one
// entity is added explicitly, only those entities are extracted.
func (b *DynamicSceneBuilder) ExtractEntity(id entity.EntityID) *DynamicSceneBuilder {
	b.only = append(b.only, id)
	return b
}

// Build performs the extraction and returns an immutable DynamicScene.
func (b *DynamicSceneBuilder) Build() DynamicScene {
	onlySet := make(map[entity.EntityID]struct{}, len(b.only))
	for _, id := range b.only {
		onlySet[id] = struct{}{}
	}

	var captured []DynamicEntity

	b.world.EachArchetype(func(arch ArchetypeView) bool {
		entities := arch.Entities()
		for row, ent := range entities {
			if len(onlySet) > 0 {
				if _, ok := onlySet[ent.ID()]; !ok {
					continue
				}
			}
			vals := arch.ComponentValues(row)
			var comps []ReflectedComponent
			for typeName, val := range vals {
				if !b.filter.included(typeName) {
					continue
				}
				reg := b.reg.ResolveByName(typeName)
				if reg == nil {
					continue // unknown type — skip
				}
				rv := reflect.ValueOf(val)
				// Deep copy: allocate new storage and copy.
				copied := reflect.New(rv.Type()).Elem()
				copied.Set(rv)
				comps = append(comps, ReflectedComponent{Reg: reg, Value: copied})
			}
			if len(comps) > 0 {
				captured = append(captured, DynamicEntity{
					ID:         ent.ID(),
					Components: comps,
				})
			}
		}
		return true
	})
	return DynamicScene{entities: captured}
}
