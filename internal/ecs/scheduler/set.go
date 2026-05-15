package scheduler

import "slices"

// SystemSet is a named group of systems. A system joins a set via
// [SystemNodeBuilder.InSet]; the set's configuration (run conditions,
// before/after edges relative to other sets) is declared once via
// [Schedule.ConfigureSet] and applied to every member at [Schedule.Build]
// time.
//
// SystemSet is a string alias both for ergonomics (callers can use
// untyped string constants) and for stable cross-test identity.
type SystemSet string

// systemSetConfig holds the per-set configuration accumulated through
// [SystemSetBuilder]. It is populated incrementally by user code and
// consumed at [Schedule.Build] time to derive per-member conditions and
// pairwise ordering edges.
type systemSetConfig struct {
	conditions []RunCondition
	beforeSets []SystemSet
	afterSets  []SystemSet
}

// SystemSetBuilder is returned by [Schedule.ConfigureSet] for declaring
// configuration that applies to every member of a [SystemSet]. Like
// [SystemNodeBuilder], the methods are chainable and the underlying
// configuration is merged across multiple ConfigureSet calls for the same
// set name.
type SystemSetBuilder struct {
	sched *Schedule
	set   SystemSet
}

// RunIf attaches a [RunCondition] to the set. Every member system is
// skipped on a given tick when any of its set-level (or own) conditions
// returns false. Multiple RunIf calls on the same set accumulate.
func (b *SystemSetBuilder) RunIf(cond RunCondition) *SystemSetBuilder {
	if cond == nil {
		return b
	}
	cfg := b.sched.setConfig(b.set)
	cfg.conditions = append(cfg.conditions, cond)
	b.sched.built = false
	return b
}

// Before declares that every member of this set must run before every
// member of other. Empty sets contribute no edges; cycles between sets
// surface as [ErrScheduleCycle] at [Schedule.Build] time.
func (b *SystemSetBuilder) Before(other SystemSet) *SystemSetBuilder {
	cfg := b.sched.setConfig(b.set)
	cfg.beforeSets = append(cfg.beforeSets, other)
	b.sched.built = false
	return b
}

// After declares that every member of this set must run after every
// member of other.
func (b *SystemSetBuilder) After(other SystemSet) *SystemSetBuilder {
	cfg := b.sched.setConfig(b.set)
	cfg.afterSets = append(cfg.afterSets, other)
	b.sched.built = false
	return b
}

// setConfig returns the per-set configuration, lazily creating it on
// first reference.
func (s *Schedule) setConfig(set SystemSet) *systemSetConfig {
	if s.setConfigs == nil {
		s.setConfigs = make(map[SystemSet]*systemSetConfig)
	}
	cfg, ok := s.setConfigs[set]
	if !ok {
		cfg = &systemSetConfig{}
		s.setConfigs[set] = cfg
	}
	return cfg
}

// membersOf returns the [SystemNodeID]s of every system that joined the
// given set. Returned in registration order.
func (s *Schedule) membersOf(set SystemSet) []SystemNodeID {
	var out []SystemNodeID
	for i, n := range s.nodes {
		if slices.Contains(n.sets, set) {
			out = append(out, SystemNodeID(i))
		}
	}
	return out
}
