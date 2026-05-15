package scheduler

import "github.com/teratron/boltengine/internal/ecs/world"

// RunCondition is a predicate evaluated before a system runs. When the
// predicate returns false the system (and any deferred work it would have
// produced) is skipped for the current tick. Conditions are pure with
// respect to the schedule executor — they MUST NOT mutate the world or
// rely on the order in which sibling conditions are evaluated.
//
// A nil [RunCondition] is treated as "always true" by [Schedule.Run] so
// callers can leave the field empty for the default behaviour.
type RunCondition func(w *world.World) bool

// Not returns a [RunCondition] that inverts cond. A nil cond is treated as
// "always true", so Not(nil) is equivalent to a predicate that always
// returns false.
func Not(cond RunCondition) RunCondition {
	return func(w *world.World) bool {
		if cond == nil {
			return false
		}
		return !cond(w)
	}
}

// And composes a list of conditions into a single predicate that returns
// true only when every input returns true. Evaluation is short-circuited
// on the first false. An empty argument list always returns true. Nil
// elements are skipped (treated as "always true").
func And(conds ...RunCondition) RunCondition {
	return func(w *world.World) bool {
		for _, c := range conds {
			if c == nil {
				continue
			}
			if !c(w) {
				return false
			}
		}
		return true
	}
}

// Or composes a list of conditions into a single predicate that returns
// true when any input returns true. Evaluation is short-circuited on the
// first true. An empty argument list returns false. Nil elements are
// skipped (treated as "always true" — i.e. they make the disjunction
// trivially true).
func Or(conds ...RunCondition) RunCondition {
	return func(w *world.World) bool {
		for _, c := range conds {
			if c == nil {
				return true
			}
			if c(w) {
				return true
			}
		}
		return false
	}
}
