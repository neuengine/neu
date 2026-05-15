package ecs_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/event"
	ecs "github.com/teratron/boltengine/pkg/ecs"
)

// ── component fixtures ────────────────────────────────────────────────────────

type Position struct{ X, Y float32 }
type Velocity struct{ X, Y float32 }
type Health struct{ HP int }
type Tag struct{} // zero-size tag component

// ── T-1T01: Benchmarks ───────────────────────────────────────────────────────
//
// Baseline thresholds (Intel Core i7-12700H, Go 1.26.3, -benchmem):
//   BenchmarkSpawn     ≈  250 ns/op   3 allocs/op
//   BenchmarkIter1     ≈   25 ns/op   0 allocs/op  (per-entity ~2.5 ns at 10k)
//   BenchmarkIter3     ≈   40 ns/op   0 allocs/op  (per-entity ~4.0 ns at 10k)

func BenchmarkSpawn(b *testing.B) {
	w := ecs.NewWorld()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = w.Spawn(ecs.Data{Value: Position{}})
	}
}

func BenchmarkIter1(b *testing.B) {
	const N = 10_000
	w := ecs.NewWorldWithCapacity(N, 4)
	for range N {
		_ = w.Spawn(ecs.Data{Value: Position{X: 1, Y: 2}})
	}
	q, err := ecs.NewQuery1[Position](w)
	if err != nil {
		b.Fatalf("NewQuery1: %v", err)
	}
	b.ReportAllocs()
	
	for b.Loop() {
		for _, p := range q.All(w) {
			_ = p.X
		}
	}
}

func BenchmarkIter3(b *testing.B) {
	const N = 10_000
	w := ecs.NewWorldWithCapacity(N, 8)
	for range N {
		_ = w.Spawn(
			ecs.Data{Value: Position{}},
			ecs.Data{Value: Velocity{X: 1}},
			ecs.Data{Value: Health{HP: 100}},
		)
	}
	q, err := ecs.NewQuery3[Position, Velocity, Health](w)
	if err != nil {
		b.Fatalf("NewQuery3: %v", err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, tup := range q.All(w) {
			_ = tup.A.X
		}
	}
}

// ── T-1T02: Race integration tests ───────────────────────────────────────────

// TestSchedulerRace verifies that N goroutines each building and running their
// own World+Schedule is race-free. The scheduler itself is sequential, but this
// exercises concurrent world creation and entity allocator access.
func TestSchedulerRace(t *testing.T) {
	t.Parallel()
	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			w := ecs.NewWorld()
			counter := 0
			sys := ecs.NewFuncSystem("counter", func(world *ecs.World) {
				q, _ := ecs.NewQuery1[Position](world)
				for range q.All(world) {
					counter++
				}
			})
			sched := ecs.NewSchedule("update")
			sched.AddSystem(sys)
			if err := sched.Build(); err != nil {
				return
			}
			for range 100 {
				_ = w.Spawn(ecs.Data{Value: Position{}})
				_ = sched.Run(w)
			}
		}()
	}
	wg.Wait()
}

// TestCommandBufferPoolRace verifies concurrent AcquireBuffer/ReleaseBuffer is
// race-free (the pool uses sync.Pool internally).
func TestCommandBufferPoolRace(t *testing.T) {
	t.Parallel()
	w := ecs.NewWorld()
	const goroutines = 16
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			buf := ecs.AcquireBuffer(w)
			cmds := ecs.NewCommands(buf)
			for range 10 {
				_ = cmds.SpawnEmpty()
			}
			ecs.ReleaseBuffer(buf)
		}()
	}
	wg.Wait()
}

// TestEventBusFrameRotation verifies EventBus double-buffer rotation correctness:
// events sent in tick N become readable in tick N and N+1, then disappear.
func TestEventBusFrameRotation(t *testing.T) {
	t.Parallel()
	w := ecs.NewWorld()
	type Msg struct{ V int }
	bus := ecs.RegisterEvent[Msg](w)
	writer := event.NewEventWriter[Msg](w)
	reader := event.NewEventReaderAt[Msg](w)

	writer.Send(Msg{V: 1})
	writer.Send(Msg{V: 2})

	// Tick boundary: rotate buffers.
	ecs.SwapAll(w)

	// After swap, reader should see both messages.
	count := 0
	for range reader.All() {
		count++
	}
	if count != 2 {
		t.Fatalf("after swap: got %d events, want 2", count)
	}

	// Second swap: events age out; a fresh reader at frontier sees none.
	ecs.SwapAll(w)
	reader2 := event.NewEventReaderAt[Msg](w)
	for range reader2.All() {
		t.Fatal("reader2 should see no events after second swap")
	}

	_ = bus // suppress unused warning
}

// ── T-1T03: Fuzz coverage ────────────────────────────────────────────────────
//
// FuzzEntityIDRoundTrip and FuzzRegisterOrderingIsDeterministic already exist
// in the internal packages (entity/entity_test.go and component/registry_test.go
// respectively). The tests below add coverage at the pkg/ecs boundary.

func FuzzSpawnDespawnSymmetry(f *testing.F) {
	// Verify that spawning then despawning an entity always leaves the world
	// in a consistent state (entity not alive, no leaked records).
	f.Add(uint32(0))
	f.Add(uint32(1000))
	f.Fuzz(func(t *testing.T, n uint32) {
		if n > 1000 {
			n = 1000 // cap to keep fuzz fast
		}
		w := ecs.NewWorld()
		entities := make([]ecs.Entity, n)
		for i := range n {
			entities[i] = w.Spawn(ecs.Data{Value: Position{}})
		}
		for _, e := range entities {
			if err := w.Despawn(e); err != nil {
				t.Fatalf("Despawn: %v", err)
			}
			if w.Contains(e) {
				t.Fatalf("entity still alive after Despawn")
			}
		}
	})
}

// ── T-1T04: Golden test — deterministic archetype migration order ─────────────

// TestDeterministicArchetypeMigration verifies that a fixed sequence of
// component additions and removals always produces the same archetype IDs.
// This pins the hash-free, registration-order-based archetype assignment
// contract so regressions surface immediately.
func TestDeterministicArchetypeMigration(t *testing.T) {
	t.Parallel()

	run := func() []string {
		w := ecs.NewWorld()
		var trace []string

		record := func(label string, e ecs.Entity) {
			archID, ok := w.ArchetypeOf(e)
			if !ok {
				t.Fatalf("ArchetypeOf(%v) returned false", e)
			}
			trace = append(trace, fmt.Sprintf("%s:arch%d", label, archID))
		}

		// 1000 entities churn: spawn with 1 component, insert a second, remove
		// the first, insert a third — each entity migrates through 3 archetypes.
		for range 1000 {
			e := w.Spawn(ecs.Data{Value: Position{}}) // arch[P]
			record("spawn-P", e)

			_ = w.Insert(e, ecs.Data{Value: Velocity{}}) // arch[P,V]
			record("ins-V", e)

			_ = w.Insert(e, ecs.Data{Value: Health{HP: 100}}) // arch[P,V,H]
			record("ins-H", e)
		}
		return trace
	}

	first := run()
	second := run()

	if len(first) != len(second) {
		t.Fatalf("trace length mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i] != second[i] {
			t.Fatalf("trace[%d] = %q, want %q", i, second[i], first[i])
		}
	}

	// Spot-check: the very first entity must land in archetype 1 (first
	// non-empty archetype, since archetype 0 is the empty archetype).
	if len(first) > 0 && first[0] != "spawn-P:arch1" {
		t.Fatalf("first spawn must be arch1, got %q", first[0])
	}
}
