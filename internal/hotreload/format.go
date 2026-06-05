//go:build editor

// Package hotreload implements development-only hot-reload: process-restart with
// a transactional World snapshot, plus in-process shader hot-swap. All of it is
// gated behind the editor build tag and excluded from production builds
// (l1-hot-reload INV-4). This file defines the snapshot wire format; encode and
// restore live in snapshot.go and restore.go.
//
// World capture reuses the scene system's reflection extraction (the
// WorldReader/ArchetypeView interfaces of l2-scene-system-go) and the
// TypeRegistry, but uses a self-contained per-entity JSON encoding rather than
// scene.SerializedScene: the latter is positional and drops the entity
// generation, whereas hot-reload MUST preserve the full EntityID (index +
// generation) for INV-5. Component values use the same JSON round-trip coercion
// the scene hydrator relies on.
package hotreload

import "encoding/json"

// CurrentSnapshotVersion is the on-disk format version. Bump on an
// incompatible layout change so a stale snapshot is rejected rather than
// misread.
const CurrentSnapshotVersion uint32 = 1

// SnapshotHeader carries the metadata needed to validate a snapshot before it
// is applied to a World (l1-hot-reload §5.3).
type SnapshotHeader struct {
	// EngineVersion is the building engine's version; a restore into a process
	// built from a different engine version is rejected (INV-1 clean-start).
	EngineVersion string `json:"engine_version"`
	// ComponentTypes lists the registered type names at snapshot time, for
	// diagnostics when a type is missing on restore.
	ComponentTypes []string `json:"component_types"`
	// Timestamp is the snapshot creation time (unix nanoseconds).
	Timestamp int64 `json:"timestamp"`
	// SnapshotVersion is the format version ([CurrentSnapshotVersion]).
	SnapshotVersion uint32 `json:"snapshot_version"`
	// EntityCount is the number of entities captured.
	EntityCount uint32 `json:"entity_count"`
}

// CompatibleWith reports whether a snapshot with this header may be restored
// into a process built from engineVersion. The format version must match
// exactly and the engine version must match (the conservative rule — a
// mismatch falls back to a clean start with an error overlay).
func (h SnapshotHeader) CompatibleWith(engineVersion string) bool {
	return h.SnapshotVersion == CurrentSnapshotVersion && h.EngineVersion == engineVersion
}

// CameraSnapshot is a portable capture of the active camera, stored as raw
// float arrays so the format does not depend on the render/camera packages.
type CameraSnapshot struct {
	// Translation is the camera world position [x, y, z].
	Translation [3]float64 `json:"translation"`
	// Rotation is the camera orientation quaternion [x, y, z, w].
	Rotation [4]float64 `json:"rotation"`
	// Viewport is the camera viewport rect [x, y, width, height].
	Viewport [4]float64 `json:"viewport"`
	// Projection holds projection parameters [fovOrSize, near, far, aspect].
	Projection [4]float64 `json:"projection"`
}

// TimeSnapshot captures elapsed time and frame count (NOT the per-frame delta,
// which is reset on resume so the first restored frame does not see a huge gap).
type TimeSnapshot struct {
	ElapsedNanos int64  `json:"elapsed_nanos"`
	FrameCount   uint64 `json:"frame_count"`
}

// AppState is the non-World portion of a snapshot: game-flow state, active
// schedules, camera, and time.
type AppState struct {
	FlowState       string         `json:"flow_state"`
	ActiveSchedules []string       `json:"active_schedules"`
	Camera          CameraSnapshot `json:"camera"`
	Time            TimeSnapshot   `json:"time"`
}

// ComponentSnapshot is one component's captured state: its fully-qualified
// type name (the TypeRegistry key) plus its JSON-encoded field values.
type ComponentSnapshot struct {
	TypeName string          `json:"type"`
	Data     json.RawMessage `json:"data"`
}

// EntitySnapshot captures one entity by its full packed EntityID (index +
// generation) and its serializable components. The ID is preserved exactly so
// restore can pin it (INV-5).
type EntitySnapshot struct {
	Components []ComponentSnapshot `json:"components"`
	ID         uint64              `json:"id"`
}

// Snapshot is the complete hot-reload state capture: a header, the captured
// entities (ID-preserving), and the app state. Dropped records component types
// skipped during capture because they are not registered/serializable (INV-2)
// so the restore side can surface them — never drop silently.
type Snapshot struct {
	App      AppState           `json:"app"`
	Entities []EntitySnapshot   `json:"entities"`
	Dropped  []DroppedComponent `json:"dropped,omitempty"`
	Header   SnapshotHeader     `json:"header"`
}

// DroppedComponent records a component type that could not be serialized,
// together with how many entities lost it (INV-2: dropped, never silent).
type DroppedComponent struct {
	TypeName     string `json:"type_name"`
	AffectedRows int    `json:"affected_rows"`
}
