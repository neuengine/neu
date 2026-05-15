package component

import "github.com/teratron/boltengine/internal/ecs/entity"

// HookContext is the opaque object passed to a [Hook] when the World fires a
// component lifecycle event. It is intentionally an empty interface: the
// concrete type (`*world.DeferredWorld`, T-1C02) is not yet defined and
// cannot be imported here without creating an import cycle. Hook authors
// should type-assert `ctx` to the concrete world type at call time.
//
// Forward declaration only — the World package will publish a richer
// interface once available; this type is a placeholder that keeps the hook
// signatures stable.
type HookContext any

// Hook fires on a component lifecycle event for a single entity.
//
// Hooks are invoked by the World during structural modification (Insert,
// Remove, Replace). They observe state but should perform structural changes
// only via the deferred-world contract supplied through ctx — direct world
// mutation from inside a hook is undefined behaviour.
type Hook func(ctx HookContext, e entity.Entity)

// Hooks groups all lifecycle callbacks for a component type. The zero value
// is a valid no-op set: missing hooks are simply skipped by the World.
//
//   - OnAdd     fires the first time the component appears on an entity.
//   - OnInsert  fires on every insertion, including overwrite.
//   - OnReplace fires when an existing component value is overwritten.
//   - OnRemove  fires just before the component is detached.
type Hooks struct {
	OnAdd     Hook
	OnInsert  Hook
	OnReplace Hook
	OnRemove  Hook
}

// Any reports whether at least one hook is set.
func (h Hooks) Any() bool {
	return h.OnAdd != nil || h.OnInsert != nil || h.OnReplace != nil || h.OnRemove != nil
}
