package ui

import "github.com/neuengine/neu/pkg/ecs"

// Interaction is the per-node pointer state, updated each frame in PreUpdate
// (INV-3). User code reads it but must not write it.
type Interaction uint8

const (
	// InteractionNone — pointer is not over the node.
	InteractionNone Interaction = iota
	// InteractionHovered — pointer is over the node.
	InteractionHovered
	// InteractionPressed — pointer is down on the node.
	InteractionPressed
)

// String renders the interaction state. Total switch.
func (i Interaction) String() string {
	switch i {
	case InteractionHovered:
		return "Hovered"
	case InteractionPressed:
		return "Pressed"
	default:
		return "None"
	}
}

// MouseFilter controls how a node participates in pointer dispatch (L1 §4.4).
type MouseFilter uint8

const (
	// MouseStop consumes pointer events (default for interactive nodes).
	MouseStop MouseFilter = iota
	// MousePass handles events AND lets them pass to nodes behind.
	MousePass
	// MouseIgnore is invisible to pointer events.
	MouseIgnore
)

// Focused marks the entity that currently holds keyboard/gamepad focus.
type Focused struct{}

// TabIndex orders automatic tab navigation. Lower indices receive focus first.
type TabIndex struct{ Index int32 }

// FocusNeighbor declares explicit directional focus targets for gamepad/keyboard
// navigation (L1 §4.12). A zero entity means "no neighbor in that direction".
type FocusNeighbor struct {
	Left, Right, Up, Down ecs.Entity
}
