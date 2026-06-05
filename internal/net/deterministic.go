package net

import (
	"math/rand/v2"
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

// DeterministicSchedule runs a fixed, explicitly-ordered set of systems at a
// fixed timestep with a reproducible per-tick RNG — the foundation both client
// prediction and lockstep build on (l1-networking-system INV-2: identical inputs
// + initial state produce identical state at every tick).
//
// Determinism comes from three choices: systems run in insertion order with NO
// DAG parallelism (a SequentialExecutor would also work; running the slice
// directly is simplest and equally ordered); the RNG is reseeded each tick from
// RngSeed and the tick number, so randomness is reproducible; and systems must
// read only Tick + FixedTimestep, never wall-clock. Reusing the deterministic
// schedule for rollback resimulation reproduces live results exactly.
type DeterministicSchedule struct {
	rng           *rand.Rand
	systems       []scheduler.System
	RngSeed       uint64
	Tick          uint64
	FixedTimestep time.Duration
}

// NewDeterministicSchedule returns a schedule seeded with rngSeed running at
// timestep. Systems are added in execution order via AddSystem.
func NewDeterministicSchedule(rngSeed uint64, timestep time.Duration) *DeterministicSchedule {
	return &DeterministicSchedule{
		RngSeed:       rngSeed,
		FixedTimestep: timestep,
		rng:           rand.New(rand.NewPCG(rngSeed, 0)),
	}
}

// AddSystem appends a system to the deterministic execution order.
func (d *DeterministicSchedule) AddSystem(s scheduler.System) {
	d.systems = append(d.systems, s)
}

// AddFunc is a convenience for adding a named function system.
func (d *DeterministicSchedule) AddFunc(name string, run func(*world.World)) {
	d.AddSystem(scheduler.NewFuncSystem(name, run))
}

// SystemCount returns the number of registered systems.
func (d *DeterministicSchedule) SystemCount() int { return len(d.systems) }

// Rand returns the schedule's RNG for the current tick. It is reseeded by
// RunTick, so a system reading it during tick N always sees the same sequence
// given the same RngSeed — never call it outside a RunTick.
func (d *DeterministicSchedule) Rand() *rand.Rand { return d.rng }

// RunTick advances the simulation one tick: it reseeds the RNG from
// RngSeed^tick (so each tick's randomness is reproducible and independent), sets
// Tick, then runs every system in insertion order against w. Input application
// is a system's concern (systems read the InputBuffer resource), keeping RunTick
// input-agnostic.
func (d *DeterministicSchedule) RunTick(w *world.World, tick uint64) {
	d.Tick = tick
	d.rng = rand.New(rand.NewPCG(d.RngSeed, tick))
	for _, s := range d.systems {
		s.Run(w)
	}
}
