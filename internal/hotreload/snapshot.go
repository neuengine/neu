//go:build editor

package hotreload

import (
	"time"

	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/worldsnap"
	"github.com/neuengine/neu/pkg/scene"
)

// BuildSnapshot extracts every entity reachable through reader into an
// ID-preserving Snapshot, wrapping the shared worldsnap capture core with the
// hot-reload engine-version header and app state. Each entity's full EntityID is
// captured (INV-5); a component whose type is not registered is recorded in
// Dropped (INV-2), never silently lost.
func BuildSnapshot(reader scene.WorldReader, reg *typereg.TypeRegistry, engineVersion string, app AppState) *Snapshot {
	ents, componentTypes, dropped := worldsnap.Capture(reader, reg)
	return &Snapshot{
		Header: SnapshotHeader{
			EngineVersion:   engineVersion,
			SnapshotVersion: CurrentSnapshotVersion,
			Timestamp:       time.Now().UnixNano(),
			EntityCount:     uint32(len(ents)),
			ComponentTypes:  componentTypes,
		},
		App:      app,
		Entities: ents,
		Dropped:  dropped,
	}
}
