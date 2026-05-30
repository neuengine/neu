package window

import "testing"

func TestWindowResolutionLogical(t *testing.T) {
	t.Parallel()
	// No override → scale 1.0.
	r := WindowResolution{PhysicalWidth: 1280, PhysicalHeight: 720}
	if w, h := r.Logical(); w != 1280 || h != 720 {
		t.Errorf("Logical (no scale) = %v×%v, want 1280×720", w, h)
	}
	// 2× DPI → logical is half physical.
	r2 := WindowResolution{PhysicalWidth: 2560, PhysicalHeight: 1440, ScaleFactorOverride: 2}
	if w, h := r2.Logical(); w != 1280 || h != 720 {
		t.Errorf("Logical (2x) = %v×%v, want 1280×720", w, h)
	}
}

func TestDefaultWindowAndDescriptor(t *testing.T) {
	t.Parallel()
	w := DefaultWindow("Game")
	if w.Title != "Game" || w.Mode != Windowed || !w.Visible || !w.Resizable {
		t.Errorf("DefaultWindow unexpected: %+v", w)
	}
	d := DescriptorFromWindow(w)
	if d.Title != "Game" || d.Resolution != w.Resolution || d.Mode != Windowed {
		t.Errorf("DescriptorFromWindow mismatch: %+v", d)
	}
}

func TestWindowDiffHasChanges(t *testing.T) {
	t.Parallel()
	var empty WindowDiff
	if empty.HasChanges() {
		t.Error("zero WindowDiff should report no changes")
	}
	title := "x"
	if !(WindowDiff{Title: &title}).HasChanges() {
		t.Error("diff with a set field should report changes")
	}
}

func TestPlatformEventKindStringTotal(t *testing.T) {
	t.Parallel()
	kinds := []PlatformEventKind{
		EventCreated, EventResized, EventMoved, EventCloseRequested, EventClosed,
		EventFocused, EventCursorEntered, EventCursorLeft, EventCursorMoved, EventScaleFactorChanged,
	}
	for _, k := range kinds {
		if k.String() == "Unknown" {
			t.Errorf("kind %d rendered as Unknown", k)
		}
	}
	if PlatformEventKind(250).String() != "Unknown" {
		t.Error("out-of-range kind should render Unknown")
	}
}

func TestCausesAppExit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		kind      PlatformEventKind
		cond      ExitCondition
		primary   bool
		remaining int
		want      bool
	}{
		{"primary close, OnPrimaryClosed", EventCloseRequested, OnPrimaryClosed, true, 1, true},
		{"non-primary close, OnPrimaryClosed", EventCloseRequested, OnPrimaryClosed, false, 1, false},
		{"last close, OnAllClosed", EventClosed, OnAllClosed, false, 0, true},
		{"not-last close, OnAllClosed", EventClosed, OnAllClosed, false, 2, false},
		{"primary close, DontExit", EventCloseRequested, DontExit, true, 0, false},
		{"non-close event", EventResized, OnPrimaryClosed, true, 0, false},
	}
	for _, c := range cases {
		if got := CausesAppExit(c.kind, c.cond, c.primary, c.remaining); got != c.want {
			t.Errorf("%s: CausesAppExit = %v, want %v", c.name, got, c.want)
		}
	}
}
