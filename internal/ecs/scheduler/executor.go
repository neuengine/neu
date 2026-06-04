package scheduler

import (
	"errors"
	"fmt"

	"github.com/neuengine/neu/internal/ecs/world"
)

// ErrSystemPanic is returned when a [System.Run] panics. The wrapped
// error carries the offending system's name and the recovered value
// formatted via fmt.Sprint. The schedule halts on the first panic.
var ErrSystemPanic = errors.New("ecs: system panicked during execution")

// ErrScheduleNotBuilt is returned by an executor when [Schedule.Build] has
// not yet been called (or last failed) on the schedule passed in. It is a
// programmer error: callers must `Build()` the schedule once before
// running it on every tick.
var ErrScheduleNotBuilt = errors.New("ecs: schedule has not been built")

// Executor runs a built [Schedule] against a [world.World]. The Phase 1
// surface is intentionally narrow — a single Run method that returns nil
// on success, [ErrSystemPanic] when a system panicked, or whatever error
// a system explicitly surfaced.
type Executor interface {
	Run(schedule *Schedule, w *world.World) error
}

// SequentialExecutor runs systems one at a time on the calling goroutine,
// in the order returned by [Schedule.Order]. It is sufficient for the
// Phase 1 POC and remains the deterministic baseline against which
// parallel executors (Phase 2+) are validated.
//
// Each system is wrapped in a recover so that a panic in one system does
// not corrupt the executor; the panic is captured into [ErrSystemPanic]
// with the system's name and returned to the caller.
type SequentialExecutor struct{}

// NewSequentialExecutor returns a SequentialExecutor. The type carries no
// state, but the constructor exists for symmetry with parallel executors
// added in later phases.
func NewSequentialExecutor() *SequentialExecutor { return &SequentialExecutor{} }

// Run executes every system in topological order. After all systems finish
// successfully, it invokes [world.World.ApplyDeferred] to flush registered
// command buffers — this is the Phase 1 tick-level apply point. Returns:
//
//   - nil when every system runs to completion and the deferred flush completes.
//   - [ErrScheduleNotBuilt] when the schedule has not been built yet.
//   - [ErrSystemPanic] (wrapped with the offending system's name) when a
//     system panics; subsequent systems are NOT executed and the deferred
//     flush is skipped to avoid compounding inconsistent state.
func (e *SequentialExecutor) Run(schedule *Schedule, w *world.World) error {
	if schedule == nil {
		return errors.New("ecs: nil schedule")
	}
	if !schedule.built {
		return ErrScheduleNotBuilt
	}
	for _, id := range schedule.order {
		node := &schedule.nodes[id]
		if !shouldRun(schedule, node, w) {
			continue
		}
		if err := runSystemSafe(node.system, w); err != nil {
			return err
		}
	}
	w.ApplyDeferred()
	return nil
}

// shouldRun reports whether the system at node should execute on this
// tick. A system runs iff every one of its own [RunCondition]s and every
// condition inherited from a joined [SystemSet] returns true. Empty
// condition lists trivially pass.
func shouldRun(schedule *Schedule, node *SystemNode, w *world.World) bool {
	for _, cond := range node.conditions {
		if cond != nil && !cond(w) {
			return false
		}
	}
	for _, set := range node.sets {
		cfg, ok := schedule.setConfigs[set]
		if !ok {
			continue
		}
		for _, cond := range cfg.conditions {
			if cond != nil && !cond(w) {
				return false
			}
		}
	}
	return true
}

// runSystemSafe invokes sys.Run, recovering panics and translating them
// into [ErrSystemPanic]. The recovered value is formatted via fmt.Sprint
// so the caller's diagnostic always carries the original message.
func runSystemSafe(sys System, w *world.World) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %q: %v", ErrSystemPanic, sys.Name(), r)
		}
	}()
	dispatchRun(sys, w)
	return nil
}

// Run executes the schedule with a [SequentialExecutor]. Convenience
// wrapper for the common case; callers needing alternate executors should
// instantiate them and call Run directly.
func (s *Schedule) Run(w *world.World) error {
	return NewSequentialExecutor().Run(s, w)
}
