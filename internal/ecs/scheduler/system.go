// Package scheduler provides system scheduling primitives for the ECS
// runtime: the [System] interface, a [Schedule] container, a [DAG] used to
// resolve execution order, and the executor surface that runs systems
// against a [world.World]. Phase 1 lands the System / Schedule / DAG layer
// (T-1E01); the sequential executor (T-1E02) and run conditions / system
// sets (T-1E03) compose on top.
package scheduler

import (
	"github.com/teratron/boltengine/internal/ecs/query"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// System is the unit of work the scheduler dispatches each tick. Names are
// the schedule's identity for ordering constraints — they must be unique
// within a single [Schedule].
type System interface {
	// Name returns a stable identifier used by [SystemNodeBuilder.Before]
	// and [SystemNodeBuilder.After] to express ordering relationships.
	Name() string
	// Run executes the system against the given world. Implementations
	// should not perform structural mutations directly when the executor
	// runs them in parallel — buffer through CommandBuffer (T-1F).
	Run(w *world.World)
}

// AccessAware is an optional interface implemented by systems that wish
// to declare their component access for conflict detection. Systems that
// do not implement it are treated as having empty access (no read or
// write) and may run alongside any other system as far as the scheduler
// is concerned. The DAG uses [query.Access.Conflicts] to add implicit
// ordering edges between conflicting systems when no explicit Before/After
// edge exists.
type AccessAware interface {
	Access() query.Access
}

// systemAccess returns the system's [query.Access], falling back to the
// zero value when the system does not implement [AccessAware].
func systemAccess(s System) query.Access {
	if a, ok := s.(AccessAware); ok {
		return a.Access()
	}
	return query.Access{}
}

// FuncSystem is a thin adapter that turns a name + Run closure (and
// optionally an [query.Access] declaration) into a [System]. It is the
// most common way to register systems before the function-injection
// machinery from T-1E03 is wired in.
type FuncSystem struct {
	run    func(*world.World)
	name   string
	access query.Access
	hasAcc bool
}

// NewFuncSystem builds a function-backed system. Use
// [FuncSystem.WithAccess] to declare component access at registration time.
func NewFuncSystem(name string, run func(*world.World)) *FuncSystem {
	return &FuncSystem{name: name, run: run}
}

// WithAccess attaches an [query.Access] declaration to the system, marking
// it as [AccessAware]. The descriptor is consumed by the scheduler to
// derive implicit ordering edges between conflicting systems.
func (f *FuncSystem) WithAccess(a query.Access) *FuncSystem {
	f.access = a
	f.hasAcc = true
	return f
}

// Name returns the system's identifier.
func (f *FuncSystem) Name() string { return f.name }

// Run invokes the user-supplied closure. A nil closure is treated as a
// no-op so users can declare access-only stubs in tests.
func (f *FuncSystem) Run(w *world.World) {
	if f.run != nil {
		f.run(w)
	}
}

// Access returns the declared [query.Access]. Reports the zero value when
// no access was attached via [FuncSystem.WithAccess].
func (f *FuncSystem) Access() query.Access {
	if !f.hasAcc {
		return query.Access{}
	}
	return f.access
}
