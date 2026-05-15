package event_test

import (
	"slices"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/event"
	"github.com/teratron/boltengine/internal/ecs/world"
)

type damageEvent struct{ Amount int }
type levelUpEvent struct{ Level int }

func collect[T any](r *event.EventReader[T]) []T {
	var out []T
	for e := range r.All() {
		out = append(out, e)
	}
	return out
}

func TestEventBus_RegisterReturnsExisting(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	a := event.RegisterEvent[damageEvent](w)
	b := event.RegisterEvent[damageEvent](w)
	if a != b {
		t.Fatal("RegisterEvent must be idempotent — return the same bus on repeated calls")
	}
}

func TestEventBus_BusBeforeRegister(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if got := event.Bus[damageEvent](w); got != nil {
		t.Fatalf("Bus must return nil before RegisterEvent, got %v", got)
	}
}

func TestEventBus_SendThenRead(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	rd := event.NewEventReader[damageEvent](w)

	wr.Send(damageEvent{Amount: 1})
	wr.Send(damageEvent{Amount: 2})
	wr.Send(damageEvent{Amount: 3})

	got := collect(rd)
	want := []damageEvent{{1}, {2}, {3}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	// Second Read yields nothing — cursor advanced past every event.
	if !rd.IsEmpty() {
		t.Fatal("reader must be empty after consuming everything")
	}
	if got := collect(rd); len(got) != 0 {
		t.Fatalf("second read returned %v, want []", got)
	}
}

func TestEventReader_TwoReadersIndependent(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	r1 := event.NewEventReader[damageEvent](w)
	r2 := event.NewEventReader[damageEvent](w)

	wr.Send(damageEvent{Amount: 10})
	wr.Send(damageEvent{Amount: 20})

	g1 := collect(r1)
	g2 := collect(r2)
	if !slices.Equal(g1, g2) || len(g1) != 2 {
		t.Fatalf("readers diverged: r1=%v r2=%v", g1, g2)
	}
}

func TestEventBus_PreviousFrameVisible(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	bus := event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)

	wr.Send(damageEvent{Amount: 1})
	wr.Send(damageEvent{Amount: 2})
	bus.Swap()
	// Reader created AFTER swap still sees previous-frame events.
	rd := event.NewEventReader[damageEvent](w)
	got := collect(rd)
	if !slices.Equal(got, []damageEvent{{1}, {2}}) {
		t.Fatalf("post-swap reader missed previous frame events: %v", got)
	}
}

func TestEventBus_LostAfterTwoSwaps(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	bus := event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	rd := event.NewEventReader[damageEvent](w)

	wr.Send(damageEvent{Amount: 1}) // frame 1
	bus.Swap()                      // frame 2 begins
	wr.Send(damageEvent{Amount: 2}) // frame 2
	bus.Swap()                      // frame 3 begins (frame-1 events now lost)
	wr.Send(damageEvent{Amount: 3}) // frame 3

	got := collect(rd)
	if !slices.Equal(got, []damageEvent{{2}, {3}}) {
		t.Fatalf("expected frame-1 events lost, frame-2 and frame-3 visible; got %v", got)
	}
}

func TestEventReader_AcrossSwap(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	bus := event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	rd := event.NewEventReader[damageEvent](w)

	wr.Send(damageEvent{Amount: 1})
	g1 := collect(rd)
	bus.Swap()
	wr.Send(damageEvent{Amount: 2})
	g2 := collect(rd)

	if !slices.Equal(g1, []damageEvent{{1}}) {
		t.Fatalf("frame 1 read = %v, want [{1}]", g1)
	}
	if !slices.Equal(g2, []damageEvent{{2}}) {
		t.Fatalf("frame 2 read = %v, want [{2}] (no re-read of frame 1)", g2)
	}
}

func TestEventBus_SendBatch(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	rd := event.NewEventReader[damageEvent](w)

	wr.SendBatch(nil) // no-op
	wr.SendBatch([]damageEvent{{1}, {2}, {3}})
	got := collect(rd)
	if !slices.Equal(got, []damageEvent{{1}, {2}, {3}}) {
		t.Fatalf("SendBatch order wrong: %v", got)
	}
}

func TestEventReader_Clear(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	wr.Send(damageEvent{Amount: 1})
	wr.Send(damageEvent{Amount: 2})

	rd := event.NewEventReader[damageEvent](w)
	rd.Clear()
	if !rd.IsEmpty() {
		t.Fatal("Clear must drain the cursor to the send frontier")
	}
	wr.Send(damageEvent{Amount: 3})
	got := collect(rd)
	if !slices.Equal(got, []damageEvent{{3}}) {
		t.Fatalf("post-Clear reader saw stale events: %v", got)
	}
}

func TestEventReader_AtFrontier(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	wr.Send(damageEvent{Amount: 1})
	wr.Send(damageEvent{Amount: 2})

	rd := event.NewEventReaderAt[damageEvent](w)
	if !rd.IsEmpty() {
		t.Fatal("frontier reader must ignore previously-sent events")
	}
	wr.Send(damageEvent{Amount: 3})
	got := collect(rd)
	if !slices.Equal(got, []damageEvent{{3}}) {
		t.Fatalf("frontier reader = %v, want [{3}]", got)
	}
}

func TestEventReader_AllStopEarly(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	wr.SendBatch([]damageEvent{{1}, {2}, {3}, {4}})
	rd := event.NewEventReader[damageEvent](w)

	taken := 0
	for e := range rd.All() {
		taken++
		if e.Amount == 2 {
			break
		}
	}
	if taken != 2 {
		t.Fatalf("early break must yield exactly 2 events; got %d", taken)
	}
	// The cursor advances past every yielded event including the one that
	// triggered the early return; remaining events are 3 and 4.
	rest := collect(rd)
	if !slices.Equal(rest, []damageEvent{{3}, {4}}) {
		t.Fatalf("post-break read = %v, want [{3} {4}]", rest)
	}
}

func TestEventBus_Diagnostics(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	bus := event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	wr.Send(damageEvent{Amount: 7})

	if bus.Len() != 1 {
		t.Fatalf("Len = %d, want 1", bus.Len())
	}
	if bus.SentCount() != 1 {
		t.Fatalf("SentCount = %d, want 1", bus.SentCount())
	}
}

func TestSwapAll_RotatesEveryRegisteredBus(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	dmgBus := event.RegisterEvent[damageEvent](w)
	_ = event.RegisterEvent[levelUpEvent](w)

	dmgWr := event.NewEventWriter[damageEvent](w)
	lvlWr := event.NewEventWriter[levelUpEvent](w)
	dmgWr.Send(damageEvent{Amount: 5})
	lvlWr.Send(levelUpEvent{Level: 1})

	event.SwapAll(w)

	if got := dmgBus.SentCount() - dmgBus.Len(); got != 0 {
		// One swap, retention covers the previous frame; SentCount must
		// remain constant and the contents must still be visible.
		t.Fatalf("damage bus drift after swap: %d", got)
	}

	dmgRd := event.NewEventReader[damageEvent](w)
	lvlRd := event.NewEventReader[levelUpEvent](w)
	if collect(dmgRd)[0].Amount != 5 {
		t.Fatal("damage bus did not retain previous-frame event after SwapAll")
	}
	if collect(lvlRd)[0].Level != 1 {
		t.Fatal("levelUp bus did not retain previous-frame event after SwapAll")
	}
}

func TestSwapAll_NoRegistry(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.SwapAll(w)    // must not panic
	event.CleanupAll(w) // must not panic
}

func TestNewEventWriter_PanicWithoutRegister(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling NewEventWriter before RegisterEvent")
		}
	}()
	w := world.NewWorld()
	_ = event.NewEventWriter[damageEvent](w)
}

func TestNewEventReader_PanicWithoutRegister(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling NewEventReader before RegisterEvent")
		}
	}()
	w := world.NewWorld()
	_ = event.NewEventReader[damageEvent](w)
}

func TestNewEventReaderAt_PanicWithoutRegister(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling NewEventReaderAt before RegisterEvent")
		}
	}()
	w := world.NewWorld()
	_ = event.NewEventReaderAt[damageEvent](w)
}

func TestRegistry_LookupAndEnsure(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if event.LookupRegistry(w) != nil {
		t.Fatal("LookupRegistry must return nil before any registration")
	}
	r1 := event.EnsureRegistry(w)
	r2 := event.EnsureRegistry(w)
	if r1 != r2 {
		t.Fatal("EnsureRegistry must be idempotent")
	}
	if event.LookupRegistry(w) != r1 {
		t.Fatal("LookupRegistry must return the registered registry")
	}
}
