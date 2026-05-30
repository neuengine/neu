package window

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/pkg/ecs"
	pkgwindow "github.com/neuengine/neu/pkg/window"
)

// ent builds a distinct test entity from a slot index.
func ent(i uint32) ecs.Entity { return entity.FromID(entity.NewEntityID(i, 0)) }

func TestHeadlessBackendLifecycle(t *testing.T) {
	t.Parallel()
	b := NewHeadlessWindowBackend()
	e := ent(1)

	h, err := b.CreateWindow(e, pkgwindow.WindowDescriptor{Title: "T"})
	if err != nil || !h.IsValid() {
		t.Fatalf("CreateWindow handle=%v err=%v", h, err)
	}
	if b.ActiveCount() != 1 {
		t.Errorf("ActiveCount = %d, want 1", b.ActiveCount())
	}

	// A diff with changes is applied; an empty diff is a no-op (no Apply call).
	title := "T2"
	_ = b.ApplyChanges(e, pkgwindow.WindowDiff{Title: &title})
	_ = b.ApplyChanges(e, pkgwindow.WindowDiff{}) // no-op
	_ = b.DestroyWindow(e)

	if b.ActiveCount() != 0 {
		t.Errorf("ActiveCount after destroy = %d, want 0", b.ActiveCount())
	}
	calls := b.Calls()
	want := []string{"Create:" + idStr(e), "Apply:" + idStr(e), "Destroy:" + idStr(e)}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call[%d] = %q, want %q", i, calls[i], want[i])
		}
	}
}

func TestHeadlessEventReplayDeterministic(t *testing.T) {
	t.Parallel()
	// INV: a scripted PlatformEvent queue replays identically across runs.
	build := func() []pkgwindow.PlatformEventKind {
		b := NewHeadlessWindowBackend()
		b.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventResized, Window: ent(1), Width: 800, Height: 600})
		b.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: ent(1)})
		var got []pkgwindow.PlatformEventKind
		for {
			frame := b.PollEvents()
			if len(frame) == 0 {
				break
			}
			for _, ev := range frame {
				got = append(got, ev.Kind)
			}
		}
		return got
	}
	first := build()
	for range 20 {
		got := build()
		if len(got) != len(first) {
			t.Fatalf("replay length drift: %v vs %v", got, first)
		}
		for i := range first {
			if got[i] != first[i] {
				t.Errorf("replay drift at %d: %v vs %v", i, got[i], first[i])
			}
		}
	}
}

func TestDiffWindow(t *testing.T) {
	t.Parallel()
	base := pkgwindow.DefaultWindow("A")

	// Unchanged → empty diff (INV-4: no ApplyChanges).
	if DiffWindow(base, base).HasChanges() {
		t.Error("identical windows should diff to no changes")
	}

	// Change title + mode only.
	cur := base
	cur.Title = "B"
	cur.Mode = pkgwindow.Fullscreen
	d := DiffWindow(base, cur)
	if d.Title == nil || *d.Title != "B" {
		t.Errorf("Title diff = %v, want B", d.Title)
	}
	if d.Mode == nil || *d.Mode != pkgwindow.Fullscreen {
		t.Errorf("Mode diff = %v, want Fullscreen", d.Mode)
	}
	if d.Resolution != nil || d.Cursor != nil {
		t.Error("unchanged fields must stay nil in the diff")
	}

	// Focused is event-driven, excluded from the diff.
	cur2 := base
	cur2.Focused = !base.Focused
	if DiffWindow(base, cur2).HasChanges() {
		t.Error("Focused change must not appear in the diff (read-only)")
	}
}

func TestPrimaryWindowINV1(t *testing.T) {
	t.Parallel()
	if err := CheckSinglePrimary(0); err != nil {
		t.Errorf("0 primaries (headless) should be valid, got %v", err)
	}
	if err := CheckSinglePrimary(1); err != nil {
		t.Errorf("1 primary should be valid, got %v", err)
	}
	if err := CheckSinglePrimary(2); err == nil {
		t.Error("2 primaries should violate INV-1")
	}

	var r PrimaryWindowRes
	e := ent(7)
	r.SetPrimary(e)
	if !r.IsPrimary(e) || !r.Set {
		t.Error("SetPrimary should record the entity")
	}
	if r.IsPrimary(ent(8)) {
		t.Error("a different entity is not the primary")
	}
	r.Clear()
	if r.Set || r.IsPrimary(e) {
		t.Error("Clear should mark primary absent")
	}
}

// idStr renders an entity's packed ID as the headless call log does.
func idStr(e ecs.Entity) string {
	return itoa(uint64(e.ID()))
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
