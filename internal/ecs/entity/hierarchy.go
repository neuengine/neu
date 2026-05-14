package entity

// ChildOf is a component that establishes a parent-child relationship between
// entities. An entity that carries ChildOf{Parent: p} is a child of p.
//
// The relationship is used by the observer system to bubble events upward
// through the hierarchy: when an observer handles an event on a child entity,
// the same trigger is propagated to p (and p's parent, and so on) unless
// StopPropagation is called in a callback.
type ChildOf struct {
	Parent Entity
}
