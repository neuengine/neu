package scene

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/entity"
)

// InstanceID identifies a set of entities spawned together from a DynamicScene.
type InstanceID uint32

// WorldWriter is a minimal interface over world.World used by the spawner.
type WorldWriter interface {
	// SpawnEmpty allocates a new entity with no components.
	SpawnEmpty() entity.Entity
	// InsertComponent adds a component value to an entity by its type.
	InsertComponent(e entity.Entity, value any) error
}

// SceneSpawner manages scene instance lifetimes. It is typically stored as a
// World resource. Not safe for concurrent use.
type SceneSpawner struct {
	instances map[InstanceID][]entity.EntityID
	next      InstanceID
}

// NewSceneSpawner creates an empty SceneSpawner.
func NewSceneSpawner() *SceneSpawner {
	return &SceneSpawner{
		instances: make(map[InstanceID][]entity.EntityID),
		next:      1,
	}
}

// InstanceEntities returns the entity IDs that belong to the given instance.
// Returns nil if the instance is unknown.
func (s *SceneSpawner) InstanceEntities(id InstanceID) []entity.EntityID {
	return s.instances[id]
}

// Spawn instantiates sc into w using the two-pass protocol (INV-5):
//   - Pass 1: allocate new entity IDs and insert component data.
//   - Pass 2: rewrite all entity.EntityID fields via a reflection walk to
//     point at the freshly allocated IDs.
//
// Returns the InstanceID that groups all spawned entities.
func (s *SceneSpawner) Spawn(w WorldWriter, sc *DynamicScene) InstanceID {
	if sc == nil || len(sc.entities) == 0 {
		return 0
	}

	// remap maps original EntityIDs → freshly allocated EntityIDs.
	remap := make(map[entity.EntityID]entity.EntityID, len(sc.entities))

	// Pass 1: allocate entities and insert component copies.
	spawned := make([]spawnedEntry, 0, len(sc.entities))
	for _, de := range sc.entities {
		newEnt := w.SpawnEmpty()
		remap[de.ID] = newEnt.ID()

		// Deep-copy each component value before handing it off.
		compCopies := make([]reflect.Value, len(de.Components))
		for i, rc := range de.Components {
			c := reflect.New(rc.Value.Type()).Elem()
			c.Set(rc.Value)
			compCopies[i] = c
		}
		spawned = append(spawned, spawnedEntry{
			entity: newEnt,
			scene:  de,
			copies: compCopies,
		})
	}

	// Pass 2: rewrite EntityID fields, then insert.
	entityIDType := reflect.TypeFor[entity.EntityID]()
	for _, se := range spawned {
		for i, comp := range se.copies {
			remapEntityIDs(comp, entityIDType, remap)
			if err := w.InsertComponent(se.entity, comp.Addr().Interface()); err != nil {
				// Skip unknown component types (forward-compat policy).
				_ = se.scene.Components[i]
			}
		}
	}

	// Register instance.
	id := s.next
	s.next++
	ids := make([]entity.EntityID, len(spawned))
	for i, se := range spawned {
		ids[i] = se.entity.ID()
	}
	s.instances[id] = ids
	return id
}

// DespawnInstance removes all entities that belong to the given instance.
// The actual world deletion is delegated through the DespawnFunc.
func (s *SceneSpawner) DespawnInstance(id InstanceID, despawn func(entity.EntityID)) {
	ids, ok := s.instances[id]
	if !ok {
		return
	}
	for _, eid := range ids {
		despawn(eid)
	}
	delete(s.instances, id)
}

// ─── internal helpers ────────────────────────────────────────────────────────

type spawnedEntry struct {
	scene  DynamicEntity
	copies []reflect.Value
	entity entity.Entity
}

// remapEntityIDs walks v recursively and replaces any entity.EntityID field
// whose value appears in the remap table.
func remapEntityIDs(v reflect.Value, entityIDType reflect.Type, remap map[entity.EntityID]entity.EntityID) {
	switch v.Kind() {
	case reflect.Struct:
		for _, field := range v.Fields() {
			remapEntityIDs(field, entityIDType, remap)
		}
	case reflect.Slice:
		for i := range v.Len() {
			remapEntityIDs(v.Index(i), entityIDType, remap)
		}
	case reflect.Pointer:
		if !v.IsNil() {
			remapEntityIDs(v.Elem(), entityIDType, remap)
		}
	default:
		if v.Type() == entityIDType && v.CanSet() {
			old := entity.EntityID(v.Uint())
			if newID, ok := remap[old]; ok {
				v.SetUint(uint64(newID))
			}
		}
	}
}
