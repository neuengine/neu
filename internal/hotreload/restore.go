//go:build editor

package hotreload

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/internal/worldsnap"
)

// ErrEngineVersionMismatch is returned by ApplySnapshot when the snapshot's
// engine/format version does not match the restoring process (INV-1: a
// mismatched snapshot is rejected for a clean start, never half-applied).
var ErrEngineVersionMismatch = errors.New("hotreload: snapshot incompatible with engine version")

// RestoreResult reports the outcome of a successful ApplySnapshot. Aliased to
// the shared worldsnap result type.
type RestoreResult = worldsnap.RestoreResult

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

// ApplySnapshot restores snap into the FRESH World w. It first enforces the
// hot-reload engine/format version guard (INV-1 — a mismatched snapshot is
// rejected before any World mutation, for a clean start), then delegates the
// ID-pinning, transactional, drop-collecting restore to the shared worldsnap
// core (INV-2/INV-5).
//
// w MUST be a fresh World whose component types are already registered.
func ApplySnapshot(w *world.World, snap *Snapshot, reg *typereg.TypeRegistry, engineVersion string) (RestoreResult, error) {
	if !snap.Header.CompatibleWith(engineVersion) {
		return RestoreResult{}, fmt.Errorf("%w: snapshot %q (fmt v%d) vs engine %q (fmt v%d)",
			ErrEngineVersionMismatch, snap.Header.EngineVersion, snap.Header.SnapshotVersion,
			engineVersion, CurrentSnapshotVersion)
	}
	return worldsnap.Restore(w, snap.Entities, reg), nil
}
