package world

import (
	"maps"
	"reflect"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
)

// SpawnWithEntity parks a pre-allocated entity in the empty archetype.
// The entity must already be alive in the EntityAllocator (reserved via
// [entity.EntityAllocator.Allocate] before this call). Used by
// [command.SpawnEmptyCommand] to honour pre-reserved entity IDs.
func (w *World) SpawnWithEntity(e entity.Entity) {
	empty := w.archetypes.get(0)
	row := len(empty.entities)
	empty.entities = append(empty.entities, e)
	w.records[e.ID()] = entityRecord{archetypeID: 0, row: row}
}

// SpawnWithEntityAndData parks a pre-allocated entity into the archetype
// matching data. If data is empty, it is equivalent to [World.SpawnWithEntity].
// Used by [command.SpawnCommand] to honour pre-reserved entity IDs.
func (w *World) SpawnWithEntityAndData(e entity.Entity, data ...component.Data) {
	if len(data) == 0 {
		w.SpawnWithEntity(e)
		return
	}
	values := w.resolveData(data)
	ids := make([]component.ID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	ids = sortIDsAscending(ids)
	arch := w.archetypes.findOrCreate(ids, w.components)
	row := w.addEntityToArchetype(arch, e, values)
	w.records[e.ID()] = entityRecord{archetypeID: arch.id, row: row}
}

// RemoveByID strips the component with the given ID from e, migrating e to
// the archetype that lacks it. Returns [ErrEntityNotAlive] if the entity is
// dead and [ErrComponentNotFound] if it does not carry the component.
// Used by [command.RemoveCommand] for runtime-typed removal.
func RemoveByID(w *World, e entity.Entity, id component.ID) error {
	if !w.entities.IsAlive(e) {
		return ErrEntityNotAlive
	}
	rec, ok := w.records[e.ID()]
	if !ok {
		return ErrEntityNotAlive
	}
	oldArch := w.archetypes.get(rec.archetypeID)
	if !oldArch.Has(id) {
		return ErrComponentNotFound
	}

	newIDs := withoutID(oldArch.componentIDs, id)
	newArch := w.archetypes.findOrCreate(newIDs, w.components)

	mergedValues := make(map[component.ID]any, len(newIDs))
	if oldArch.table != nil {
		for k, v := range oldArch.table.RowValues(rec.row) {
			if k != id {
				mergedValues[k] = v
			}
		}
	}

	info := w.components.Info(id)
	if info.Storage == component.StorageSparseSet {
		if ss, ok := w.sparseSets[id]; ok {
			ss.Remove(e)
		}
	}

	w.removeEntityFromArchetype(oldArch, e, rec.row, false)
	row := w.addEntityToArchetype(newArch, e, mergedValues)
	w.records[e.ID()] = entityRecord{archetypeID: newArch.id, row: row}
	return nil
}

// Spawn creates a new entity carrying the supplied components. Component
// types are auto-registered (default StorageTable) and any required-component
// dependencies declared via [component.RequiredComponents] are auto-injected
// with zero values. The entity is placed into the archetype matching its full
// component set.
func (w *World) Spawn(data ...component.Data) entity.Entity {
	e := w.entities.Allocate()

	values := w.resolveData(data)
	ids := make([]component.ID, 0, len(values))
	for id := range values {
		ids = append(ids, id)
	}
	ids = sortIDsAscending(ids)

	arch := w.archetypes.findOrCreate(ids, w.components)
	row := w.addEntityToArchetype(arch, e, values)
	w.records[e.ID()] = entityRecord{archetypeID: arch.id, row: row}
	return e
}

// Insert adds or overwrites components on an existing entity. Adding a new
// component triggers an archetype migration; overwriting an existing
// component mutates storage in place. Returns [ErrEntityNotAlive] if the
// entity has been despawned.
func (w *World) Insert(e entity.Entity, data ...component.Data) error {
	if !w.entities.IsAlive(e) {
		return ErrEntityNotAlive
	}
	if len(data) == 0 {
		return nil
	}

	rec := w.records[e.ID()]
	oldArch := w.archetypes.get(rec.archetypeID)

	newValues := w.resolveData(data)

	// Compute the merged sorted ID set.
	allIDs := append([]component.ID(nil), oldArch.componentIDs...)
	for id := range newValues {
		allIDs = withID(allIDs, id)
	}

	newArch := w.archetypes.findOrCreate(allIDs, w.components)

	if newArch.id == oldArch.id {
		for id, val := range newValues {
			w.writeComponentValue(oldArch, rec.row, e, id, val)
		}
		return nil
	}

	// Migration: extract the kept table values, swap-and-pop the old
	// archetype, then add a row in the new archetype.
	keepSparse := make(map[component.ID]struct{}, len(newArch.componentIDs))
	for _, id := range newArch.componentIDs {
		keepSparse[id] = struct{}{}
	}

	mergedValues := make(map[component.ID]any, len(newArch.componentIDs))
	if oldArch.table != nil {
		for k, v := range oldArch.table.RowValues(rec.row) {
			if _, kept := keepSparse[k]; kept {
				mergedValues[k] = v
			}
		}
	}
	maps.Copy(mergedValues, newValues)

	w.removeEntityFromArchetype(oldArch, e, rec.row, false)
	// SparseSet components shared with the new archetype stay in the global
	// sparse set; only those *not* in the new archetype were removed above.

	row := w.addEntityToArchetype(newArch, e, mergedValues)
	w.records[e.ID()] = entityRecord{archetypeID: newArch.id, row: row}
	return nil
}

// Get returns a typed pointer to component T on the given entity, or
// (nil,false) if the entity is not alive or does not carry T. Zero-size (tag)
// components return (nil,true) — the component is present but has no payload.
func Get[T any](w *World, e entity.Entity) (*T, bool) {
	if !w.entities.IsAlive(e) {
		return nil, false
	}
	id, ok := w.components.Lookup(reflect.TypeFor[T]())
	if !ok {
		return nil, false
	}
	rec := w.records[e.ID()]
	arch := w.archetypes.get(rec.archetypeID)
	if !arch.Has(id) {
		return nil, false
	}
	info := w.components.Info(id)
	if info.Storage == component.StorageSparseSet {
		ss, ok := w.sparseSets[id]
		if !ok {
			return nil, false
		}
		ptr, present := ss.Get(e)
		if !present {
			return nil, false
		}
		if ptr == nil { // zero-size component
			return nil, true
		}
		return (*T)(ptr), true
	}
	if arch.table == nil {
		return nil, false
	}
	ptr, hasCol := arch.table.CellPtrByID(id, rec.row)
	if !hasCol {
		return nil, false
	}
	if ptr == nil {
		return nil, true
	}
	return (*T)(ptr), true
}

// Remove strips component T from the given entity, migrating it to the
// archetype without T. Returns [ErrEntityNotAlive] if the entity is dead and
// [ErrComponentNotFound] if it does not carry T.
func Remove[T any](w *World, e entity.Entity) error {
	if !w.entities.IsAlive(e) {
		return ErrEntityNotAlive
	}
	id, ok := w.components.Lookup(reflect.TypeFor[T]())
	if !ok {
		return ErrComponentNotFound
	}

	rec := w.records[e.ID()]
	oldArch := w.archetypes.get(rec.archetypeID)
	if !oldArch.Has(id) {
		return ErrComponentNotFound
	}

	newIDs := withoutID(oldArch.componentIDs, id)
	newArch := w.archetypes.findOrCreate(newIDs, w.components)

	// Carry over the kept table values (excluding the removed ID).
	mergedValues := make(map[component.ID]any, len(newIDs))
	if oldArch.table != nil {
		for k, v := range oldArch.table.RowValues(rec.row) {
			if k != id {
				mergedValues[k] = v
			}
		}
	}

	// If the removed component is sparse-set stored, evict it explicitly —
	// removeEntityFromArchetype only evicts sparse-set IDs that are *not* in
	// the new archetype, which is what we want for migration.
	info := w.components.Info(id)
	if info.Storage == component.StorageSparseSet {
		if ss, ok := w.sparseSets[id]; ok {
			ss.Remove(e)
		}
	}

	w.removeEntityFromArchetype(oldArch, e, rec.row, false)
	row := w.addEntityToArchetype(newArch, e, mergedValues)
	w.records[e.ID()] = entityRecord{archetypeID: newArch.id, row: row}
	return nil
}

// resolveData registers the component types referenced by `data`, builds a
// map from component ID to value, and auto-injects required dependencies
// using zero values when not already supplied.
func (w *World) resolveData(data []component.Data) map[component.ID]any {
	result := make(map[component.ID]any, len(data)*2)
	for _, d := range data {
		if d.Value == nil {
			continue
		}
		t := reflect.TypeOf(d.Value)
		id := w.components.RegisterByType(t)
		result[id] = d.Value
	}
	// RequiredBy is transitively resolved at registration time, so a single
	// pass over each provided ID covers the full closure.
	for _, d := range data {
		if d.Value == nil {
			continue
		}
		id, _ := w.components.Lookup(reflect.TypeOf(d.Value))
		for _, reqID := range w.components.Info(id).RequiredBy {
			if _, ok := result[reqID]; ok {
				continue
			}
			reqInfo := w.components.Info(reqID)
			result[reqID] = reflect.Zero(reqInfo.Type).Interface()
		}
	}
	return result
}

// addEntityToArchetype inserts e into arch, populating the table row and any
// sparse-set components for which `values` provides a payload. SparseSet IDs
// that already hold a value (from a prior archetype) are left untouched.
// Returns the row index assigned to the entity (also the index in
// arch.entities).
func (w *World) addEntityToArchetype(arch *Archetype, e entity.Entity, values map[component.ID]any) int {
	// Sparse-set components first — adding to the global sparse set does not
	// affect the archetype row.
	for _, id := range arch.componentIDs {
		info := w.components.Info(id)
		if info.Storage != component.StorageSparseSet {
			continue
		}
		val, supplied := values[id]
		if !supplied {
			// Already present in the global sparse set from before; leave it.
			continue
		}
		ss := w.sparseSetFor(id, info)
		ss.Add(e, val)
	}

	row := len(arch.entities)
	if arch.table != nil {
		tableValues := make(map[component.ID]any, len(values))
		for id, v := range values {
			info := w.components.Info(id)
			if info.Storage == component.StorageTable {
				tableValues[id] = v
			}
		}
		row = arch.table.AddRow(tableValues)
	}
	arch.entities = append(arch.entities, e)
	return row
}

// removeEntityFromArchetype evicts e at row from arch's table and entities
// list using swap-and-pop. When evictSparse is true, it also removes the
// entity from every sparse-set component associated with this archetype
// (used for full Despawn). The moved entity (if any) has its record updated
// to point at the new row.
func (w *World) removeEntityFromArchetype(arch *Archetype, e entity.Entity, row int, evictSparse bool) {
	last := len(arch.entities) - 1

	movedFrom := -1
	if arch.table != nil {
		movedFrom = arch.table.RemoveRow(row)
	} else if row != last {
		movedFrom = last
	}

	if movedFrom != -1 {
		moved := arch.entities[movedFrom]
		rec := w.records[moved.ID()]
		rec.row = row
		w.records[moved.ID()] = rec
	}

	if row != last {
		arch.entities[row] = arch.entities[last]
	}
	arch.entities = arch.entities[:last]

	if evictSparse {
		for _, id := range arch.componentIDs {
			info := w.components.Info(id)
			if info.Storage != component.StorageSparseSet {
				continue
			}
			if ss, ok := w.sparseSets[id]; ok {
				ss.Remove(e)
			}
		}
	}
}

// writeComponentValue overwrites a single component's storage in place. Used
// when [World.Insert] receives a value for a component the entity already
// owns (no archetype migration required).
func (w *World) writeComponentValue(arch *Archetype, row int, e entity.Entity, id component.ID, val any) {
	info := w.components.Info(id)
	if info.Storage == component.StorageSparseSet {
		ss := w.sparseSetFor(id, info)
		ss.Add(e, val) // Add overwrites if the entity already has a slot.
		return
	}
	if arch.table != nil {
		arch.table.SetCellByID(id, row, val)
	}
}

// sparseSetFor returns the global sparse set for id, lazily creating it on
// first use.
func (w *World) sparseSetFor(id component.ID, info *component.Info) *component.SparseSet {
	if ss, ok := w.sparseSets[id]; ok {
		return ss
	}
	ss := component.NewSparseSet(component.ColumnSpecFromInfo(info))
	w.sparseSets[id] = ss
	return ss
}
