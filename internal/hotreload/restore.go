//go:build editor

package hotreload

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
)

// ErrEngineVersionMismatch is returned by ApplySnapshot when the snapshot's
// engine/format version does not match the restoring process (INV-1: a
// mismatched snapshot is rejected for a clean start, never half-applied).
var ErrEngineVersionMismatch = errors.New("hotreload: snapshot incompatible with engine version")

// EncodeSnapshot serializes a snapshot to JSON bytes for the .hot-reload file.
func EncodeSnapshot(snap *Snapshot) ([]byte, error) { return json.Marshal(snap) }

// DecodeSnapshot parses snapshot bytes. A malformed snapshot fails HERE, before
// ApplySnapshot touches the World — so corrupt input degrades to a clean start
// rather than a half-mutated World (INV-1).
func DecodeSnapshot(data []byte) (*Snapshot, error) {
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, fmt.Errorf("hotreload: decode snapshot: %w", err)
	}
	return &snap, nil
}

// RestoreResult reports the outcome of a successful ApplySnapshot.
type RestoreResult struct {
	// Dropped lists component types present in the snapshot but no longer
	// registered/decodable in this process (renamed or removed since capture).
	Dropped []DroppedComponent
	// Restored is the number of entities parked into the World.
	Restored int
}

// ApplySnapshot restores snap into the FRESH World w, pinning every entity at
// its exact snapshot EntityID (index + generation) so cached Entity handles
// survive the process restart (INV-5). Guarantees:
//
//   - INV-1 (transactional): a version-incompatible snapshot is rejected before
//     any World mutation; a corrupt snapshot is already rejected at
//     DecodeSnapshot. Component construction runs in a staging pass before any
//     entity is parked, so a structural failure cannot leave a half-built World.
//   - INV-2 (explicit drops): a component whose type is no longer registered, or
//     whose data no longer decodes, is collected in RestoreResult.Dropped — never
//     silently lost.
//   - INV-5 (ID pinning): because IDs are pinned rather than remapped,
//     inter-entity EntityID references stay valid with no rewrite pass (the
//     "IdentityRemapper" is the identity function).
//
// w MUST be a fresh World whose component types are already registered (the new
// process registers them at plugin Build). engineVersion is this process's
// version (e.g. from pkg/version).
func ApplySnapshot(w *world.World, snap *Snapshot, reg *typereg.TypeRegistry, engineVersion string) (RestoreResult, error) {
	if !snap.Header.CompatibleWith(engineVersion) {
		return RestoreResult{}, fmt.Errorf("%w: snapshot %q (fmt v%d) vs engine %q (fmt v%d)",
			ErrEngineVersionMismatch, snap.Header.EngineVersion, snap.Header.SnapshotVersion,
			engineVersion, CurrentSnapshotVersion)
	}

	comp := w.Components()
	dropped := map[string]int{}

	// Staging pass: construct every entity's component data in memory first. No
	// World mutation happens until this completes, so an error here aborts
	// cleanly (INV-1). Per-component type-missing is tolerated (INV-2).
	type staged struct {
		e    entity.Entity
		data []component.Data
	}
	all := make([]staged, 0, len(snap.Entities))
	for i := range snap.Entities {
		es := &snap.Entities[i]
		id := entity.EntityID(es.ID)
		if id.IsNull() {
			continue // never restore the null sentinel
		}
		var datas []component.Data
		for _, cs := range es.Components {
			treg := reg.ResolveByName(cs.TypeName)
			if treg == nil {
				dropped[cs.TypeName]++ // type removed/renamed since capture
				continue
			}
			ptr := reflect.New(treg.Type)
			if err := json.Unmarshal(cs.Data, ptr.Interface()); err != nil {
				dropped[cs.TypeName]++ // layout changed → data no longer decodes
				continue
			}
			cid, ok := comp.Lookup(treg.Type)
			if !ok {
				dropped[cs.TypeName]++ // not in the component registry this build
				continue
			}
			datas = append(datas, component.Data{Value: ptr.Elem().Interface(), ID: cid})
		}
		all = append(all, staged{e: entity.FromID(id), data: datas})
	}

	// Apply pass: pin IDs in the allocator, rebuild the free list from the gaps
	// (INV-5), then park each entity with its components into the World.
	alloc := w.Entities()
	for _, s := range all {
		alloc.PlaceAt(s.e.Index(), s.e.Generation())
	}
	alloc.RebuildFreeList()
	for _, s := range all {
		w.SpawnWithEntityAndData(s.e, s.data...)
	}

	return RestoreResult{Restored: len(all), Dropped: droppedList(dropped)}, nil
}
