// Package hierarchy implements parent-child entity relationships, transform
// propagation, and tree traversal utilities.
package hierarchy

import (
	"iter"
	"slices"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
)

// ChildOf is placed on a child entity to declare its parent.
// An entity may have at most one ChildOf component (tree invariant, not DAG).
type ChildOf struct {
	Parent entity.Entity
}

// Children is automatically maintained on parent entities.
// Read-only for user systems — mutate via hierarchy commands.
type Children struct {
	entities []entity.Entity
}

// Slice returns a defensive copy of the child entity list.
func (c *Children) Slice() []entity.Entity {
	out := make([]entity.Entity, len(c.entities))
	copy(out, c.entities)
	return out
}

// Len returns the number of direct children.
func (c *Children) Len() int { return len(c.entities) }

// Contains reports whether e is a direct child.
func (c *Children) Contains(e entity.Entity) bool {
	return slices.Contains(c.entities, e)
}

// ── Deferred hierarchy commands ───────────────────────────────────────────────

// AddChild enqueues a deferred command that makes child a child of parent.
// Performs cycle detection and handles existing-parent removal before inserting.
func AddChild(cmds *command.Commands, parent, child entity.Entity) {
	cmds.Add(command.NewCustomCommand(func(w *world.World) {
		addChildImmediate(w, parent, child)
	}))
}

// SetParent changes the parent of child. Removes the previous parent link first.
// Equivalent to RemoveParent + AddChild in a single deferred step.
func SetParent(cmds *command.Commands, child, newParent entity.Entity) {
	cmds.Add(command.NewCustomCommand(func(w *world.World) {
		addChildImmediate(w, newParent, child)
	}))
}

// RemoveParent removes the ChildOf component from child, making it a root entity.
func RemoveParent(cmds *command.Commands, child entity.Entity) {
	cmds.Add(command.NewCustomCommand(func(w *world.World) {
		removeParentImmediate(w, child)
	}))
}

// DespawnRecursive despawns entity and all of its descendants in depth-first
// (leaves first, then ancestors) order.
func DespawnRecursive(cmds *command.Commands, e entity.Entity) {
	cmds.Add(command.NewCustomCommand(func(w *world.World) {
		despawnRecursiveImmediate(w, e)
	}))
}

// ── Immediate (apply-point) operations ───────────────────────────────────────

func addChildImmediate(w *world.World, parent, child entity.Entity) {
	if !w.Contains(parent) || !w.Contains(child) {
		return
	}
	if parent == child {
		return // self-parenting
	}
	// Cycle check: walk ancestor chain from parent upward.
	if wouldCreateCycle(w, parent, child) {
		return
	}

	// Remove from previous parent if already parented.
	if co, ok := world.Get[ChildOf](w, child); ok {
		unlinkChild(w, co.Parent, child)
	}

	// Insert ChildOf on child.
	_ = w.Insert(child, component.Data{Value: ChildOf{Parent: parent}})

	// Update parent's Children.
	if ch, ok := world.Get[Children](w, parent); ok {
		ch.entities = append(ch.entities, child)
	} else {
		_ = w.Insert(parent, component.Data{Value: Children{entities: []entity.Entity{child}}})
	}
}

func removeParentImmediate(w *world.World, child entity.Entity) {
	co, ok := world.Get[ChildOf](w, child)
	if !ok {
		return
	}
	unlinkChild(w, co.Parent, child)
	_ = world.Remove[ChildOf](w, child)
}

func unlinkChild(w *world.World, parent, child entity.Entity) {
	ch, ok := world.Get[Children](w, parent)
	if !ok {
		return
	}
	for i, e := range ch.entities {
		if e == child {
			ch.entities = append(ch.entities[:i], ch.entities[i+1:]...)
			break
		}
	}
}

func despawnRecursiveImmediate(w *world.World, e entity.Entity) {
	if !w.Contains(e) {
		return
	}
	if ch, ok := world.Get[Children](w, e); ok {
		for _, v := range slices.Backward(ch.entities) {
			despawnRecursiveImmediate(w, v)
		}
	}
	_ = w.Despawn(e)
}

func wouldCreateCycle(w *world.World, proposedParent, child entity.Entity) bool {
	cur := proposedParent
	for {
		if cur == child {
			return true
		}
		co, ok := world.Get[ChildOf](w, cur)
		if !ok {
			return false
		}
		cur = co.Parent
	}
}

// ── Traversal ─────────────────────────────────────────────────────────────────

// ChildrenOf iterates the direct children of e. Empty if e has no Children.
func ChildrenOf(w *world.World, e entity.Entity) iter.Seq[entity.Entity] {
	return func(yield func(entity.Entity) bool) {
		ch, ok := world.Get[Children](w, e)
		if !ok {
			return
		}
		for _, child := range ch.entities {
			if !yield(child) {
				return
			}
		}
	}
}

// Descendants returns a depth-first iterator over all descendants of e
// (not including e itself).
func Descendants(w *world.World, e entity.Entity) iter.Seq[entity.Entity] {
	return func(yield func(entity.Entity) bool) {
		descendantsInner(w, e, yield)
	}
}

func descendantsInner(w *world.World, e entity.Entity, yield func(entity.Entity) bool) bool {
	ch, ok := world.Get[Children](w, e)
	if !ok {
		return true
	}
	for _, child := range ch.entities {
		if !yield(child) {
			return false
		}
		if !descendantsInner(w, child, yield) {
			return false
		}
	}
	return true
}

// Ancestors walks from e's parent upward to the root (not including e).
func Ancestors(w *world.World, e entity.Entity) iter.Seq[entity.Entity] {
	return func(yield func(entity.Entity) bool) {
		cur := e
		for {
			co, ok := world.Get[ChildOf](w, cur)
			if !ok {
				return
			}
			if !yield(co.Parent) {
				return
			}
			cur = co.Parent
		}
	}
}

// Root returns the root ancestor of e. Returns e itself if it has no parent.
func Root(w *world.World, e entity.Entity) entity.Entity {
	cur := e
	for {
		co, ok := world.Get[ChildOf](w, cur)
		if !ok {
			return cur
		}
		cur = co.Parent
	}
}

// IsDescendantOf reports whether e is a descendant of ancestor.
func IsDescendantOf(w *world.World, e, ancestor entity.Entity) bool {
	cur := e
	for {
		co, ok := world.Get[ChildOf](w, cur)
		if !ok {
			return false
		}
		if co.Parent == ancestor {
			return true
		}
		cur = co.Parent
	}
}
