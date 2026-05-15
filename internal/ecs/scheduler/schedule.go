package scheduler

import (
	"errors"
	"fmt"

	"github.com/teratron/boltengine/internal/ecs/query"
)

// ErrDuplicateSystem is returned by [Schedule.AddSystem] when a name is
// reused within the same schedule.
var ErrDuplicateSystem = errors.New("ecs: duplicate system name in schedule")

// ErrUnknownSystem is returned by [Schedule.Build] when an explicit
// Before/After constraint references a system that was never added.
var ErrUnknownSystem = errors.New("ecs: unknown system referenced in ordering constraint")

// SystemNode pairs a [System] with its scheduling metadata: declared
// component access (T-1E01), run conditions, and set memberships (both
// added in T-1E03).
type SystemNode struct {
	system     System
	conditions []RunCondition
	sets       []SystemSet
	access     query.Access
	id         SystemNodeID
}

// ID returns the node's index in the schedule.
func (n *SystemNode) ID() SystemNodeID { return n.id }

// System returns the underlying [System].
func (n *SystemNode) System() System { return n.system }

// Access returns the node's declared component access.
func (n *SystemNode) Access() query.Access { return n.access }

// Conditions returns the run conditions attached directly to this node.
// Set-level conditions are NOT included here — they are evaluated by the
// executor by walking the node's [SystemNode.Sets].
func (n *SystemNode) Conditions() []RunCondition { return n.conditions }

// Sets returns the [SystemSet]s this node has joined.
func (n *SystemNode) Sets() []SystemSet { return n.sets }

// Schedule is a named, ordered collection of [System]s. Construct it with
// [NewSchedule], register systems with [Schedule.AddSystem] (chained with
// [SystemNodeBuilder.Before] / [SystemNodeBuilder.After]), and call
// [Schedule.Build] to derive the topological execution order.
//
// Build draws edges from three sources:
//
//  1. Explicit Before/After constraints declared on each node.
//  2. Implicit access conflicts: when two systems' [query.Access] sets
//     conflict and neither has an explicit ordering toward the other, the
//     scheduler inserts an edge in registration order (earlier system runs
//     first). This keeps the schedule deterministic without requiring
//     callers to enumerate every pairwise constraint.
//  3. Self-loops (from == to) are rejected as cycles, surfacing as
//     [ErrScheduleCycle].
type Schedule struct {
	dag        *DAG
	nameToID   map[string]SystemNodeID
	setConfigs map[SystemSet]*systemSetConfig
	name       string
	nodes      []SystemNode
	beforeRefs []orderingRef
	afterRefs  []orderingRef
	order      []SystemNodeID
	built      bool
}

// orderingRef captures a deferred Before/After constraint referenced by
// the target system's name. Names are resolved to IDs at [Schedule.Build]
// time so users can declare constraints in any order, including against
// systems added later.
type orderingRef struct {
	target string
	source SystemNodeID
}

// NewSchedule creates an empty schedule.
func NewSchedule(name string) *Schedule {
	return &Schedule{
		name:     name,
		nameToID: make(map[string]SystemNodeID),
	}
}

// Name returns the schedule's identifier (e.g. "Update", "FixedUpdate").
func (s *Schedule) Name() string { return s.name }

// SystemCount returns the number of systems registered with the schedule.
func (s *Schedule) SystemCount() int { return len(s.nodes) }

// AddSystem registers sys with the schedule and returns a builder for
// declaring ordering constraints. The system's [System.Name] must be
// unique within the schedule; duplicates yield [ErrDuplicateSystem] via
// the builder's [SystemNodeBuilder.Err].
func (s *Schedule) AddSystem(sys System) *SystemNodeBuilder {
	if sys == nil {
		return &SystemNodeBuilder{err: errors.New("ecs: nil system")}
	}
	name := sys.Name()
	if _, dup := s.nameToID[name]; dup {
		return &SystemNodeBuilder{err: fmt.Errorf("%w: %q", ErrDuplicateSystem, name)}
	}
	id := SystemNodeID(len(s.nodes))
	s.nodes = append(s.nodes, SystemNode{
		id:     id,
		system: sys,
		access: systemAccess(sys),
	})
	s.nameToID[name] = id
	s.built = false
	return &SystemNodeBuilder{sched: s, id: id}
}

// Build resolves explicit ordering constraints, inserts implicit edges
// from [query.Access] conflicts, and topologically sorts the resulting
// graph. After a successful Build, [Schedule.Order] returns the execution
// order; [Schedule.Run] (T-1E02) consumes it.
func (s *Schedule) Build() error {
	dag := NewDAG(len(s.nodes))

	// 1. Explicit Before edges: source must run BEFORE target.
	for _, ref := range s.beforeRefs {
		target, ok := s.nameToID[ref.target]
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownSystem, ref.target)
		}
		if ref.source == target {
			return cycleError([]SystemNodeID{ref.source})
		}
		dag.AddEdge(ref.source, target)
	}
	// 2. Explicit After edges: target must run BEFORE source.
	for _, ref := range s.afterRefs {
		target, ok := s.nameToID[ref.target]
		if !ok {
			return fmt.Errorf("%w: %q", ErrUnknownSystem, ref.target)
		}
		if ref.source == target {
			return cycleError([]SystemNodeID{ref.source})
		}
		dag.AddEdge(target, ref.source)
	}

	// 3. Set-level Before/After: expand each (setA → setB) into pairwise
	// edges between every member of setA and every member of setB. Empty
	// sets contribute nothing; cycles surface as ErrScheduleCycle below.
	for set, cfg := range s.setConfigs {
		members := s.membersOf(set)
		if len(members) == 0 {
			continue
		}
		for _, other := range cfg.beforeSets {
			for _, otherID := range s.membersOf(other) {
				for _, ownID := range members {
					if ownID == otherID {
						return cycleError([]SystemNodeID{ownID})
					}
					dag.AddEdge(ownID, otherID)
				}
			}
		}
		for _, other := range cfg.afterSets {
			for _, otherID := range s.membersOf(other) {
				for _, ownID := range members {
					if ownID == otherID {
						return cycleError([]SystemNodeID{ownID})
					}
					dag.AddEdge(otherID, ownID)
				}
			}
		}
	}

	// 4. Implicit Access-conflict edges. Walk pairs in registration order;
	// if the systems' Access sets conflict and neither node already has an
	// explicit edge between them, add an edge from the earlier-registered
	// system to the later one. This makes the schedule deterministic
	// without forcing callers to enumerate every pairwise constraint.
	for i := 0; i < len(s.nodes); i++ {
		for j := i + 1; j < len(s.nodes); j++ {
			a, b := s.nodes[i].access, s.nodes[j].access
			if !a.Conflicts(b) {
				continue
			}
			from, to := SystemNodeID(i), SystemNodeID(j)
			if dag.HasEdge(from, to) || dag.HasEdge(to, from) {
				continue
			}
			dag.AddEdge(from, to)
		}
	}

	if err := dag.Build(); err != nil {
		return err
	}
	s.dag = dag
	s.order = dag.TopologicalOrder()
	s.built = true
	return nil
}

// Order returns the topologically sorted [SystemNodeID]s after a
// successful [Schedule.Build]. Returns nil before Build.
func (s *Schedule) Order() []SystemNodeID {
	if !s.built {
		return nil
	}
	out := make([]SystemNodeID, len(s.order))
	copy(out, s.order)
	return out
}

// SystemsInOrder returns the systems in topological order. Convenience
// wrapper over [Schedule.Order]. Returns nil before [Schedule.Build].
func (s *Schedule) SystemsInOrder() []System {
	if !s.built {
		return nil
	}
	out := make([]System, len(s.order))
	for i, id := range s.order {
		out[i] = s.nodes[id].system
	}
	return out
}

// Node returns the [SystemNode] with the given ID. Out-of-range IDs panic.
func (s *Schedule) Node(id SystemNodeID) *SystemNode {
	return &s.nodes[id]
}

// DAG exposes the underlying [DAG] for inspection (used by tests and the
// Phase 1 sequential executor in T-1E02). Returns nil before Build.
func (s *Schedule) DAG() *DAG {
	if !s.built {
		return nil
	}
	return s.dag
}

// ConfigureSet returns a [SystemSetBuilder] for declaring run conditions
// and ordering relationships that apply to every member of the named set.
// Multiple ConfigureSet calls for the same set name accumulate.
func (s *Schedule) ConfigureSet(set SystemSet) *SystemSetBuilder {
	s.setConfig(set) // ensure the entry exists
	return &SystemSetBuilder{sched: s, set: set}
}

// SystemNodeBuilder is returned by [Schedule.AddSystem] for chaining
// ordering constraints. Errors raised during construction (duplicate name,
// nil system) are deferred to [SystemNodeBuilder.Err] and to
// [Schedule.Build].
type SystemNodeBuilder struct {
	sched *Schedule
	err   error
	id    SystemNodeID
}

// ID returns the assigned [SystemNodeID]. Returns the zero value if the
// builder is in an error state.
func (b *SystemNodeBuilder) ID() SystemNodeID { return b.id }

// Err returns the deferred construction error (e.g. duplicate name).
// Returns nil when the system was registered successfully.
func (b *SystemNodeBuilder) Err() error { return b.err }

// Before declares that this system must run before the system identified
// by targetName. The reference is resolved at [Schedule.Build] time, so
// targetName may name a system that is added later.
func (b *SystemNodeBuilder) Before(targetName string) *SystemNodeBuilder {
	if b.err != nil {
		return b
	}
	b.sched.beforeRefs = append(b.sched.beforeRefs, orderingRef{source: b.id, target: targetName})
	b.sched.built = false
	return b
}

// After declares that this system must run after the system identified
// by targetName. Like [SystemNodeBuilder.Before], the reference is
// resolved at [Schedule.Build] time.
func (b *SystemNodeBuilder) After(targetName string) *SystemNodeBuilder {
	if b.err != nil {
		return b
	}
	b.sched.afterRefs = append(b.sched.afterRefs, orderingRef{source: b.id, target: targetName})
	b.sched.built = false
	return b
}

// RunIf attaches a [RunCondition] directly to this system. The system is
// skipped on a tick when this condition (or any condition inherited from
// a [SystemSet] it belongs to) returns false. Multiple RunIf calls
// accumulate; passing nil is a no-op.
func (b *SystemNodeBuilder) RunIf(cond RunCondition) *SystemNodeBuilder {
	if b.err != nil || cond == nil {
		return b
	}
	b.sched.nodes[b.id].conditions = append(b.sched.nodes[b.id].conditions, cond)
	return b
}

// InSet adds the system to the named [SystemSet]. The system inherits any
// conditions and set-level Before/After constraints declared on the set
// via [Schedule.ConfigureSet]. A system may join multiple sets; duplicate
// joins to the same set are folded silently.
func (b *SystemNodeBuilder) InSet(set SystemSet) *SystemNodeBuilder {
	if b.err != nil {
		return b
	}
	for _, existing := range b.sched.nodes[b.id].sets {
		if existing == set {
			return b
		}
	}
	b.sched.nodes[b.id].sets = append(b.sched.nodes[b.id].sets, set)
	b.sched.built = false
	return b
}
