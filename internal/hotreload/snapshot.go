//go:build editor

package hotreload

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/pkg/scene"
)

// BuildSnapshot extracts every entity reachable through reader into an
// ID-preserving Snapshot. Each entity's full EntityID is captured (INV-5) and
// each component is JSON-serialized under its registry type name. A component
// whose type is not registered in reg cannot be restored, so it is recorded in
// Dropped (INV-2) rather than silently lost. Component-less entities are still
// captured so their IDs survive the round-trip.
//
// reader and the ArchetypeView it yields are the scene system's reflection
// extraction interfaces (l2-scene-system-go), reused here so hot-reload does not
// duplicate World traversal.
func BuildSnapshot(reader scene.WorldReader, reg *typereg.TypeRegistry, engineVersion string, app AppState) *Snapshot {
	snap := &Snapshot{
		Header: SnapshotHeader{
			EngineVersion:   engineVersion,
			SnapshotVersion: CurrentSnapshotVersion,
			Timestamp:       time.Now().UnixNano(),
		},
		App: app,
	}
	dropped := map[string]int{}
	typeSet := map[string]struct{}{}

	reader.EachArchetype(func(arch scene.ArchetypeView) bool {
		ents := arch.Entities()
		for row, ent := range ents {
			es := EntitySnapshot{ID: uint64(ent.ID())}
			for typeName, val := range arch.ComponentValues(row) {
				if reg.ResolveByName(typeName) == nil {
					dropped[typeName]++ // not registered → can't restore (INV-2)
					continue
				}
				data, err := json.Marshal(val)
				if err != nil {
					dropped[typeName]++
					continue
				}
				es.Components = append(es.Components, ComponentSnapshot{TypeName: typeName, Data: data})
				typeSet[typeName] = struct{}{}
			}
			snap.Entities = append(snap.Entities, es)
		}
		return true
	})

	snap.Header.EntityCount = uint32(len(snap.Entities))
	snap.Header.ComponentTypes = sortedKeys(typeSet)
	snap.Dropped = droppedList(dropped)
	return snap
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
