//go:build editor

package hotreload

import (
	"encoding/json"
	"testing"
)

func TestSnapshotHeaderCompatibility(t *testing.T) {
	t.Parallel()
	h := SnapshotHeader{
		EngineVersion:   "2.1.28",
		SnapshotVersion: CurrentSnapshotVersion,
	}
	if !h.CompatibleWith("2.1.28") {
		t.Error("matching engine + format version should be compatible")
	}
	if h.CompatibleWith("2.2.0") {
		t.Error("engine version mismatch must be incompatible (clean-start, INV-1)")
	}
	stale := SnapshotHeader{EngineVersion: "2.1.28", SnapshotVersion: CurrentSnapshotVersion + 1}
	if stale.CompatibleWith("2.1.28") {
		t.Error("format version mismatch must be incompatible")
	}
}

func TestSnapshotJSONRoundTrip(t *testing.T) {
	t.Parallel()
	snap := Snapshot{
		Header: SnapshotHeader{
			EngineVersion:   "2.1.28",
			SnapshotVersion: CurrentSnapshotVersion,
			Timestamp:       1_700_000_000,
			EntityCount:     3,
			ComponentTypes:  []string{"Position", "Velocity"},
		},
		App: AppState{
			FlowState:       "InGame",
			ActiveSchedules: []string{"Update", "FixedUpdate"},
			Camera: CameraSnapshot{
				Translation: [3]float64{1, 2, 3},
				Rotation:    [4]float64{0, 0, 0, 1},
				Viewport:    [4]float64{0, 0, 1920, 1080},
				Projection:  [4]float64{60, 0.1, 1000, 1.777},
			},
			Time: TimeSnapshot{ElapsedNanos: 5_000_000, FrameCount: 300},
		},
		Dropped: []DroppedComponent{{TypeName: "GPUHandle", AffectedRows: 2}},
	}

	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Snapshot
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Header.EngineVersion != "2.1.28" || got.Header.EntityCount != 3 {
		t.Errorf("header round-trip mismatch: %+v", got.Header)
	}
	if got.App.FlowState != "InGame" || len(got.App.ActiveSchedules) != 2 {
		t.Errorf("app state round-trip mismatch: %+v", got.App)
	}
	if got.App.Camera.Translation != [3]float64{1, 2, 3} || got.App.Time.FrameCount != 300 {
		t.Errorf("camera/time round-trip mismatch: %+v", got.App)
	}
	if len(got.Dropped) != 1 || got.Dropped[0].TypeName != "GPUHandle" || got.Dropped[0].AffectedRows != 2 {
		t.Errorf("dropped round-trip mismatch: %+v", got.Dropped)
	}
}
