package event_test

import (
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
)

type pingMsg struct{ Seq int }
type pongMsg struct{ Seq int }

func collectMsgs[T any](r *event.MessageReader[T]) []T {
	var out []T
	for m := range r.All() {
		out = append(out, m)
	}
	return out
}

func TestMessageChannel_RegisterReturnsExisting(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	a := event.RegisterMessage[pingMsg](w, 16)
	b := event.RegisterMessage[pingMsg](w, 32) // capacity ignored on subsequent calls
	if a != b {
		t.Fatal("RegisterMessage must be idempotent")
	}
	if a.Capacity() != 16 {
		t.Fatalf("Capacity = %d, want 16 (first registration wins)", a.Capacity())
	}
}

func TestMessageChannel_DefaultCapacity(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	ch := event.RegisterMessage[pingMsg](w, 0)
	if ch.Capacity() != 64 {
		t.Fatalf("default capacity = %d, want 64", ch.Capacity())
	}
}

func TestMessageChannel_BeforeRegister(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if got := event.Channel[pingMsg](w); got != nil {
		t.Fatalf("Channel before RegisterMessage = %v, want nil", got)
	}
}

func TestMessageChannel_WriteRead(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 16)
	rd := event.NewMessageReader[pingMsg](w)
	wr := event.NewMessageWriter[pingMsg](w)

	wr.Write(pingMsg{Seq: 1})
	wr.Write(pingMsg{Seq: 2})
	wr.Write(pingMsg{Seq: 3})

	got := collectMsgs(rd)
	want := []pingMsg{{1}, {2}, {3}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if !rd.IsEmpty() {
		t.Fatal("reader must be empty after consuming everything")
	}
}

func TestMessageChannel_SecondReadIsEmpty(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 16)
	rd := event.NewMessageReader[pingMsg](w)
	wr := event.NewMessageWriter[pingMsg](w)

	wr.Write(pingMsg{Seq: 1})
	collectMsgs(rd)
	if got := collectMsgs(rd); len(got) != 0 {
		t.Fatalf("second Read returned %v, want []", got)
	}
}

func TestMessageReader_Independent(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 16)
	wr := event.NewMessageWriter[pingMsg](w)
	r1 := event.NewMessageReader[pingMsg](w)
	r2 := event.NewMessageReader[pingMsg](w)

	wr.Write(pingMsg{Seq: 5})
	wr.Write(pingMsg{Seq: 6})

	g1 := collectMsgs(r1)
	g2 := collectMsgs(r2)
	if !slices.Equal(g1, g2) || len(g1) != 2 {
		t.Fatalf("readers diverged: r1=%v r2=%v", g1, g2)
	}
}

func TestMessageReader_StartsAtCurrentHead(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 16)
	wr := event.NewMessageWriter[pingMsg](w)

	wr.Write(pingMsg{Seq: 1})
	wr.Write(pingMsg{Seq: 2})

	// Reader registered AFTER writes should see only future messages.
	rd := event.NewMessageReader[pingMsg](w)
	if !rd.IsEmpty() {
		t.Fatal("post-write reader must not see prior messages")
	}
	wr.Write(pingMsg{Seq: 3})
	got := collectMsgs(rd)
	if !slices.Equal(got, []pingMsg{{3}}) {
		t.Fatalf("got %v, want [{3}]", got)
	}
}

func TestMessageChannel_RingWrapLossy(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 4)
	rd := event.NewMessageReader[pingMsg](w)
	wr := event.NewMessageWriter[pingMsg](w)

	// Write 6 messages into a 4-slot ring without reading; oldest 2 are
	// overwritten by the writer.
	for i := 1; i <= 6; i++ {
		wr.Write(pingMsg{Seq: i})
	}
	got := collectMsgs(rd)
	want := []pingMsg{{3}, {4}, {5}, {6}}
	if !slices.Equal(got, want) {
		t.Fatalf("ring wrap: got %v, want %v (oldest 2 must be lost)", got, want)
	}
}

func TestMessageChannel_FullCapacityNoLoss(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 4)
	rd := event.NewMessageReader[pingMsg](w)
	wr := event.NewMessageWriter[pingMsg](w)

	for i := 1; i <= 4; i++ {
		wr.Write(pingMsg{Seq: i})
	}
	got := collectMsgs(rd)
	want := []pingMsg{{1}, {2}, {3}, {4}}
	if !slices.Equal(got, want) {
		t.Fatalf("at-capacity read: got %v, want %v", got, want)
	}
}

func TestMessageChannel_HeadAndReaderCount(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	ch := event.RegisterMessage[pingMsg](w, 8)
	wr := event.NewMessageWriter[pingMsg](w)
	wr.Write(pingMsg{Seq: 1})
	wr.Write(pingMsg{Seq: 2})

	if ch.Head() != 2 {
		t.Fatalf("Head = %d, want 2", ch.Head())
	}

	r1 := event.NewMessageReader[pingMsg](w)
	r2 := event.NewMessageReader[pingMsg](w)
	if ch.ReaderCount() != 2 {
		t.Fatalf("ReaderCount = %d, want 2", ch.ReaderCount())
	}
	r1.Close()
	if ch.ReaderCount() != 1 {
		t.Fatalf("ReaderCount after Close = %d, want 1", ch.ReaderCount())
	}
	// Reader after Close must yield nothing.
	if got := collectMsgs(r1); len(got) != 0 {
		t.Fatalf("closed reader returned %v, want []", got)
	}
	r2.Close()
}

func TestMessageReader_AllStopEarly(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 16)
	wr := event.NewMessageWriter[pingMsg](w)
	for i := 1; i <= 4; i++ {
		wr.Write(pingMsg{Seq: i})
	}
	rd := event.NewMessageReader[pingMsg](w)
	// Reader was registered AFTER writes — re-write so it sees 4 messages.
	for i := 1; i <= 4; i++ {
		wr.Write(pingMsg{Seq: i})
	}

	taken := 0
	for m := range rd.All() {
		taken++
		if m.Seq == 2 {
			break
		}
	}
	if taken != 2 {
		t.Fatalf("early break must yield exactly 2 messages; got %d", taken)
	}
	// Cursor advanced past the yielded message; remaining should be 3 and 4.
	rest := collectMsgs(rd)
	if !slices.Equal(rest, []pingMsg{{3}, {4}}) {
		t.Fatalf("post-break read = %v, want [{3} {4}]", rest)
	}
}

func TestMessageReader_LenAccountsForLost(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 4)
	rd := event.NewMessageReader[pingMsg](w)
	wr := event.NewMessageWriter[pingMsg](w)
	for i := 1; i <= 6; i++ {
		wr.Write(pingMsg{Seq: i})
	}
	if got := rd.Len(); got != 4 {
		t.Fatalf("Len after lossy wrap = %d, want 4 (capacity)", got)
	}
}

func TestCleanupAll_NoOpForRingChannels(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 8)
	event.RegisterMessage[pongMsg](w, 8)
	event.CleanupAll(w) // must not panic
}

func TestNewMessageWriter_PanicWithoutRegister(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	w := world.NewWorld()
	_ = event.NewMessageWriter[pingMsg](w)
}

func TestNewMessageReader_PanicWithoutRegister(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	w := world.NewWorld()
	_ = event.NewMessageReader[pingMsg](w)
}
