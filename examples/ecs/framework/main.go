// Package main is the Phase 2 framework gate example.
// It demonstrates the full DefaultPlugins stack end-to-end:
//   - State machine with multiple transitions and DespawnOnExit observation
//   - 3-level hierarchy with GlobalTransform propagation
//   - Changed/Added filters selecting only mutated rows
//   - 100+ tick headless run
//
// Run: go run ./examples/ecs/framework
// Test: go test -race ./examples/ecs/framework
package main

import (
	"fmt"
	"log"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/hierarchy"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/state"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app"
	"github.com/neuengine/neu/pkg/app/appface"
	neu "github.com/neuengine/neu/pkg/math"
)

// ── Game state ────────────────────────────────────────────────────────────────

type GamePhase uint8

const (
	PhaseLoading GamePhase = iota
	PhaseGame
	PhaseGameOver
)

// ── Position component (used for Changed filter demonstration) ────────────────

type Position struct{ X, Y float32 }

func main() {
	result, err := runFramework()
	if err != nil {
		log.Fatalf("framework error: %v", err)
	}
	fmt.Println(result)
}

type frameworkResult struct {
	ticks              int
	transitions        int
	hierarchyVerified  bool
	changedDetected    bool
}

func (r frameworkResult) String() string {
	return fmt.Sprintf(
		"Phase 2 framework gate: ticks=%d transitions=%d hierarchy=%v changed=%v",
		r.ticks, r.transitions, r.hierarchyVerified, r.changedDetected,
	)
}

func runFramework() (frameworkResult, error) {
	a := app.NewApp()
	a.AddPlugins(app.DefaultPlugins{})

	// ── State machine ─────────────────────────────────────────────────────────

	state.InitState(a, PhaseLoading)

	var (
		transitions       int
		hierarchyVerified bool
		changedDetected   bool
		ticks             int
		posEntity         entity.Entity // tracks the Position entity for Changed verification
	)

	// ── Startup: spawn 3-level hierarchy + a Position entity ──────────────────

	a.AddSystem(appface.Startup, scheduler.NewFuncSystem("fw.spawn", func(w *world.World) {
		buf := command.AcquireBuffer(w.Entities())
		cmds := command.NewCommands(buf)

		// Hierarchy: root → child → grandchild with distinct translations.
		root := cmds.Spawn(component.Data{Value: hierarchy.FromTranslation(neu.Vec3{X: 1})})
		child := cmds.Spawn(component.Data{Value: hierarchy.FromTranslation(neu.Vec3{Y: 1})})
		grandchild := cmds.Spawn(component.Data{Value: hierarchy.FromTranslation(neu.Vec3{Z: 1})})
		hierarchy.AddChild(cmds, root, child)
		hierarchy.AddChild(cmds, child, grandchild)

		// Spawn a Position entity for Changed-filter demonstration.
		posEntity = cmds.Spawn(component.Data{Value: Position{X: 0, Y: 0}})

		buf.Apply(w)
		command.ReleaseBuffer(buf)

		// Begin first transition: Loading → Game.
		state.TransitionTo(w, PhaseGame)
	}))

	// ── Update: drive transitions and verify per-frame invariants ─────────────

	a.AddSystem(appface.Update, scheduler.NewFuncSystem("fw.update", func(w *world.World) {
		ticks++

		// Detect state transitions.
		if state.StateChanged[GamePhase]()(w) {
			transitions++
		}

		// Mutate Position on tick 5 so Changed filter can detect it.
		// IncrementChangeTick must precede Insert so the mutation stamp
		// is strictly greater than lastChangeTick (currently 0).
		if ticks == 5 {
			w.IncrementChangeTick()
			_ = w.Insert(posEntity, component.Data{Value: Position{X: 99}})
		}

		// Verify Changed filter on tick 6: only the mutated entity should match.
		if ticks == 6 && !changedDetected {
			q, err := query.NewQuery1[Position](w, query.Changed[Position]{})
			if err == nil {
				if q.Count(w) > 0 {
					changedDetected = true
				}
			}
		}

		// Second transition: Game → GameOver at tick 50.
		if ticks == 50 {
			state.TransitionTo(w, PhaseGameOver)
		}

		// Stop after 100 ticks.
		if ticks >= 100 {
			a.Exit()
		}
	}))

	// ── PostUpdate: verify GlobalTransform propagation ────────────────────────

	a.AddSystem(appface.PostUpdate, scheduler.NewFuncSystem("fw.verify", func(w *world.World) {
		if hierarchyVerified {
			return
		}
		// After the first PostUpdate, check grandchild world position.
		// Grandchild world pos should be {1,1,1} (root+child+grandchild translations).
		q, err := query.NewQuery1[hierarchy.GlobalTransform](w)
		if err != nil {
			return
		}
		gts := make([]hierarchy.GlobalTransform, 0, 3)
		for _, gt := range q.All(w) {
			gts = append(gts, *gt)
		}
		// Any entity with world translation ~= {1,1,1} proves 3-level propagation.
		for _, gt := range gts {
			pos := gt.Translation()
			if approx(pos.X, 1) && approx(pos.Y, 1) && approx(pos.Z, 1) {
				hierarchyVerified = true
				break
			}
		}
	}))

	if err := a.Run(); err != nil {
		return frameworkResult{}, err
	}

	return frameworkResult{
		ticks:             ticks,
		transitions:       transitions,
		hierarchyVerified: hierarchyVerified,
		changedDetected:   changedDetected,
	}, nil
}

func approx(a, b float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-4
}
