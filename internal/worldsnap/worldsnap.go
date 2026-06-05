// Package worldsnap is the shared, build-tag-free core for capturing a World to
// an ID-preserving byte form and restoring it with the entity IDs pinned exactly
// (index + generation). It is consumed by two subsystems that must NOT depend on
// each other or on each other's build tags:
//
//   - internal/hotreload (//go:build editor) — process-restart state snapshots,
//     wrapping Capture/Restore with an engine-version header + app state.
//   - internal/net (runtime) — the SnapshotManager for rollback/lockstep,
//     wrapping them with a tick + CRC32 checksum.
//
// Both share one tested capture/restore path rather than duplicating World
// traversal + entity-ID pinning. The pinning primitive itself
// ([entity.EntityAllocator.PlaceAt]/RebuildFreeList) is already build-tag-free,
// so only the encode/restore logic lives here.
package worldsnap

import (
	"encoding/json"
	"reflect"
	"sort"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/scene"
)

// ComponentSnapshot is one component's captured state: its fully-qualified type
// name (the TypeRegistry key) plus its JSON-encoded field values.
type ComponentSnapshot struct {
	TypeName string          `json:"type"`
	Data     json.RawMessage `json:"data"`
}

// EntitySnapshot captures one entity by its full packed EntityID (index +
// generation) and its serializable components. The ID is preserved exactly so
// Restore can pin it.
type EntitySnapshot struct {
	Components []ComponentSnapshot `json:"components"`
	ID         uint64              `json:"id"`
}

// DroppedComponent records a component type skipped during capture/restore
// because it is not registered/serializable, with how many entities it affected.
// Dropped, never silent.
type DroppedComponent struct {
	TypeName     string `json:"type_name"`
	AffectedRows int    `json:"affected_rows"`
}

// RestoreResult reports the outcome of a Restore.
type RestoreResult struct {
	Dropped  []DroppedComponent
	Restored int
}

// Capture extracts every entity reachable through reader into ID-preserving
// snapshots. Each entity's full EntityID is captured and each component is
// JSON-serialized under its registry type name; a component whose type is not
// registered in reg cannot be restored, so it is recorded in the dropped list
// (never silently lost). Component-less entities are still captured so their IDs
// survive. componentTypes is the sorted set of captured type names (for a
// caller's header). reader is the scene system's reflection extraction interface.
func Capture(reader scene.WorldReader, reg *typereg.TypeRegistry) (ents []EntitySnapshot, componentTypes []string, dropped []DroppedComponent) {
	dropTally := map[string]int{}
	typeSet := map[string]struct{}{}

	reader.EachArchetype(func(arch scene.ArchetypeView) bool {
		for row, ent := range arch.Entities() {
			es := EntitySnapshot{ID: uint64(ent.ID())}
			for typeName, val := range arch.ComponentValues(row) {
				if reg.ResolveByName(typeName) == nil {
					dropTally[typeName]++ // not registered → can't restore
					continue
				}
				data, err := json.Marshal(val)
				if err != nil {
					dropTally[typeName]++
					continue
				}
				es.Components = append(es.Components, ComponentSnapshot{TypeName: typeName, Data: data})
				typeSet[typeName] = struct{}{}
			}
			ents = append(ents, es)
		}
		return true
	})
	return ents, sortedKeys(typeSet), droppedList(dropTally)
}

// Restore applies entity snapshots into the FRESH World w, pinning every entity
// at its exact snapshot EntityID (index + generation) so cached Entity handles
// survive. Because IDs are pinned rather than remapped, inter-entity EntityID
// references stay valid with no rewrite pass. Component construction runs in a
// staging pass before any World mutation, so a structural failure cannot leave a
// half-built World; a component whose type is no longer registered or no longer
// decodes is collected in RestoreResult.Dropped (never silent).
//
// w MUST be a fresh World whose component types are already registered.
func Restore(w *world.World, ents []EntitySnapshot, reg *typereg.TypeRegistry) RestoreResult {
	comp := w.Components()
	dropTally := map[string]int{}

	type staged struct {
		e    entity.Entity
		data []component.Data
	}
	all := make([]staged, 0, len(ents))
	for i := range ents {
		id := entity.EntityID(ents[i].ID)
		if id.IsNull() {
			continue // never restore the null sentinel
		}
		var datas []component.Data
		for _, cs := range ents[i].Components {
			treg := reg.ResolveByName(cs.TypeName)
			if treg == nil {
				dropTally[cs.TypeName]++ // type removed/renamed since capture
				continue
			}
			ptr := reflect.New(treg.Type)
			if err := json.Unmarshal(cs.Data, ptr.Interface()); err != nil {
				dropTally[cs.TypeName]++ // layout changed → data no longer decodes
				continue
			}
			cid, ok := comp.Lookup(treg.Type)
			if !ok {
				dropTally[cs.TypeName]++ // not in the component registry this build
				continue
			}
			datas = append(datas, component.Data{Value: ptr.Elem().Interface(), ID: cid})
		}
		all = append(all, staged{e: entity.FromID(id), data: datas})
	}

	// Apply pass: pin IDs in the allocator, rebuild the free list from the gaps,
	// then park each entity with its components.
	alloc := w.Entities()
	for _, s := range all {
		alloc.PlaceAt(s.e.Index(), s.e.Generation())
	}
	alloc.RebuildFreeList()
	for _, s := range all {
		w.SpawnWithEntityAndData(s.e, s.data...)
	}

	return RestoreResult{Restored: len(all), Dropped: droppedList(dropTally)}
}

// sortedKeys returns the map keys in deterministic ascending order so a snapshot
// of identical state is byte-stable.
func sortedKeys(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// droppedList converts the dropped-type tally into a sorted slice.
func droppedList(m map[string]int) []DroppedComponent {
	if len(m) == 0 {
		return nil
	}
	out := make([]DroppedComponent, 0, len(m))
	for name, n := range m {
		out = append(out, DroppedComponent{TypeName: name, AffectedRows: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TypeName < out[j].TypeName })
	return out
}
