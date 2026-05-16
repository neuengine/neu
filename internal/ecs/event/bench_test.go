package event_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/world"
)

// BenchmarkEventSend measures Send into a pre-grown bus. After warm-up the
// backing slice has capacity for the per-batch fill, so each Send is a
// no-realloc append.
func BenchmarkEventSend(b *testing.B) {
	w := world.NewWorld()
	bus := event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		wr.Send(damageEvent{Amount: 1})
		// Drain so the slice doesn't grow unboundedly.
		if bus.Len() >= 1024 {
			bus.Swap()
			bus.Swap()
		}
	}
}

// BenchmarkEventReadDrain measures iteration cost over 100 events and
// resetting via Swap. After warm-up the iterator allocates nothing.
func BenchmarkEventReadDrain(b *testing.B) {
	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	wr := event.NewEventWriter[damageEvent](w)
	rd := event.NewEventReader[damageEvent](w)

	b.ReportAllocs()

	for b.Loop() {
		for range 100 {
			wr.Send(damageEvent{Amount: 1})
		}
		var sum int
		for e := range rd.All() {
			sum += e.Amount
		}
		_ = sum
	}
}

// BenchmarkMessageWrite measures ring-buffer Write into a 256-slot channel.
// After warm-up the operation is a single index/write; no allocations.
func BenchmarkMessageWrite(b *testing.B) {
	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 256)
	wr := event.NewMessageWriter[pingMsg](w)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		wr.Write(pingMsg{Seq: i})
	}
}

// BenchmarkMessageReadDrain measures cursor-based iteration on a 256-slot
// channel after writing 100 messages. After warm-up the iterator allocates
// nothing.
func BenchmarkMessageReadDrain(b *testing.B) {
	w := world.NewWorld()
	event.RegisterMessage[pingMsg](w, 256)
	wr := event.NewMessageWriter[pingMsg](w)
	rd := event.NewMessageReader[pingMsg](w)

	b.ReportAllocs()

	for b.Loop() {
		for j := range 100 {
			wr.Write(pingMsg{Seq: j})
		}
		var sum int
		for m := range rd.All() {
			sum += m.Seq
		}
		_ = sum
	}
}

// BenchmarkSwapAll measures the cost of rotating every registered bus on a
// World with 4 event types. After warm-up, no allocations.
func BenchmarkSwapAll(b *testing.B) {
	w := world.NewWorld()
	event.RegisterEvent[damageEvent](w)
	event.RegisterEvent[levelUpEvent](w)
	event.RegisterEvent[pingMsg](w)
	event.RegisterEvent[pongMsg](w)

	b.ReportAllocs()

	for b.Loop() {
		event.SwapAll(w)
	}
}
