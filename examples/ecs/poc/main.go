// Command poc demonstrates a complete ECS tick loop using the public API.
//
// Setup:
//   - 10 000 entities across 3 archetypes (ArchPV, ArchPH, ArchPVH).
//   - 3 systems in a DAG: Movement → Combat → Collector.
//   - Commands and events round-trip through one tick boundary.
//
// Run:  go run ./examples/ecs/poc
// Test: go test -race ./examples/ecs/poc
package main

import (
	"fmt"

	ecs "github.com/teratron/boltengine/pkg/ecs"
)

// ── component types ───────────────────────────────────────────────────────────

type Position struct{ X, Y float32 }
type Velocity struct{ DX, DY float32 }
type Health struct{ HP int }

// ── event types ───────────────────────────────────────────────────────────────

type MoveEvent struct{ Count int }

// ── state shared across systems (stored as World resource) ───────────────────

type Stats struct {
	Ticks      int
	Moved      int
	Damaged    int
	EventsRecv int
	Spawned    int
}

func main() {
	const (
		archPVCount  = 3_000 // Position + Velocity
		archPHCount  = 2_000 // Position + Health
		archPVHCount = 5_000 // Position + Velocity + Health
		ticks        = 100
	)

	w := ecs.NewWorldWithCapacity(archPVCount+archPHCount+archPVHCount+ticks+16, 8)

	// Register event bus before any system uses it.
	ecs.RegisterEvent[MoveEvent](w)

	// Seed world stats resource.
	ecs.SetResource(w, Stats{})

	// Spawn entities across 3 archetypes.
	for range archPVCount {
		_ = w.Spawn(
			ecs.Data{Value: Position{}},
			ecs.Data{Value: Velocity{DX: 0.1, DY: 0.05}},
		)
	}
	for range archPHCount {
		_ = w.Spawn(
			ecs.Data{Value: Position{}},
			ecs.Data{Value: Health{HP: 100}},
		)
	}
	for range archPVHCount {
		_ = w.Spawn(
			ecs.Data{Value: Position{}},
			ecs.Data{Value: Velocity{DX: 0.05, DY: 0.1}},
			ecs.Data{Value: Health{HP: 50}},
		)
	}

	// ── queries (built once, reused every tick) ───────────────────────────────

	qPV, err := ecs.NewQuery2[Position, Velocity](w)
	if err != nil {
		panic(err)
	}
	qH, err := ecs.NewQuery1[Health](w)
	if err != nil {
		panic(err)
	}

	// ── event reader positioned at the frontier (sees only future events) ─────
	reader := ecs.NewEventReaderAt[MoveEvent](w)

	// ── systems ───────────────────────────────────────────────────────────────

	// MovementSystem: integrate velocity into position; send one MoveEvent per tick.
	moveSys := ecs.NewFuncSystem("MovementSystem", func(world *ecs.World) {
		writer := ecs.NewEventWriter[MoveEvent](world)
		moved := 0
		for _, tup := range qPV.All(world) {
			tup.A.X += tup.B.DX
			tup.A.Y += tup.B.DY
			moved++
		}
		writer.Send(MoveEvent{Count: moved})
		if s, ok := ecs.Resource[Stats](world); ok {
			s.Moved += moved
		}
	})

	// CombatSystem: deal 1 damage per tick; use a command to spawn a new entity
	// when health reaches 0 — round-trips through the apply-point.
	combatBuf := ecs.AcquireBuffer(w)
	combatBuf.RegisterWith(w)
	combatCmds := ecs.NewCommands(combatBuf)

	combatSys := ecs.NewFuncSystem("CombatSystem", func(world *ecs.World) {
		damaged := 0
		for e, h := range qH.All(world) {
			h.HP--
			damaged++
			if h.HP <= 0 {
				// Command: despawn dead entity (applied at tick boundary).
				combatCmds.Despawn(e)
				// Command: spawn a replacement (round-trip proof).
				_ = combatCmds.Spawn(ecs.Data{Value: Health{HP: 10}})
				if s, ok := ecs.Resource[Stats](world); ok {
					s.Spawned++
				}
			}
		}
		if s, ok := ecs.Resource[Stats](world); ok {
			s.Damaged += damaged
		}
	})

	// CollectorSystem: drain move events from the PREVIOUS tick's send.
	collectSys := ecs.NewFuncSystem("CollectorSystem", func(world *ecs.World) {
		recv := 0
		for range reader.All() {
			recv++
		}
		if s, ok := ecs.Resource[Stats](world); ok {
			s.EventsRecv += recv
		}
	})

	// ── schedule: Movement → Combat → Collector ───────────────────────────────

	sched := ecs.NewSchedule("Update")
	sched.AddSystem(moveSys).Before("CombatSystem")
	sched.AddSystem(combatSys).Before("CollectorSystem")
	sched.AddSystem(collectSys)
	if err := sched.Build(); err != nil {
		panic(fmt.Sprintf("schedule build: %v", err))
	}

	// ── main tick loop ────────────────────────────────────────────────────────

	for tick := range ticks {
		// Rotate event buffers: current-tick writes become "previous" for readers.
		ecs.SwapAll(w)

		if err := sched.Run(w); err != nil {
			panic(fmt.Sprintf("tick %d: %v", tick, err))
		}

		if s, ok := ecs.Resource[Stats](w); ok {
			s.Ticks++
		}
	}

	// ── release pool-rented buffer ────────────────────────────────────────────

	// combatBuf is registered as a deferred flusher; release after the loop.
	// Note: releasing here is safe because no more ticks will run.
	w.ResetDeferredFlushers()
	ecs.ReleaseBuffer(combatBuf)

	// ── print results ─────────────────────────────────────────────────────────

	s, _ := ecs.Resource[Stats](w)

	q1, _ := ecs.NewQuery1[Position](w)
	totalEntities := q1.Count(w)

	fmt.Printf("ticks:        %d\n", s.Ticks)
	fmt.Printf("entities:     %d\n", totalEntities)
	fmt.Printf("moved:        %d\n", s.Moved)
	fmt.Printf("damaged:      %d\n", s.Damaged)
	fmt.Printf("events_recv:  %d\n", s.EventsRecv)
	fmt.Printf("spawned:      %d\n", s.Spawned)
}
