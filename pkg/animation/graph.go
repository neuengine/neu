package animation

import (
	"errors"

	"github.com/neuengine/neu/pkg/asset"
)

// ErrAnimGraphCycle is returned when an AnimationGraph's transition topology
// contains an illegal cycle (non-looping edge that forms a cycle).
var ErrAnimGraphCycle = errors.New("animation: illegal cycle in graph transitions")

// nodeKind discriminates AnimationNode variants.
type nodeKind uint8

const (
	kindClip  nodeKind = iota
	kindBlend          // 1D parameter blend of children
	kindAdd            // additive layer over a base
)

// AnimationNode is the sealed interface for graph nodes.
type AnimationNode interface {
	kind() nodeKind
}

// ClipNode plays a single AnimationClip.
type ClipNode struct{ Clip asset.Handle[AnimationClip] }

func (ClipNode) kind() nodeKind { return kindClip }

// BlendNode mixes child nodes based on a named float parameter.
// Weights are computed from the parameter value and per-child thresholds.
type BlendNode struct {
	Param      string             // parameter name in AnimationGraph.Params
	Children   []AnimationNodeIndex
	Thresholds []float32 // one per child; len must equal len(Children)
}

func (BlendNode) kind() nodeKind { return kindBlend }

// AddNode additively layers Additive on top of Base (e.g. breathing overlay).
type AddNode struct {
	Base     AnimationNodeIndex
	Additive AnimationNodeIndex
}

func (AddNode) kind() nodeKind { return kindAdd }

// TransitionCondition selects what triggers a state transition.
type TransitionCondition uint8

const (
	// TransitionThreshold fires when a named parameter crosses a threshold.
	TransitionThreshold TransitionCondition = iota
	// TransitionTrigger fires when a named trigger parameter is set.
	TransitionTrigger
	// TransitionClipFinished fires when the source clip completes.
	TransitionClipFinished
)

// Transition describes an edge in the animation graph state machine.
type Transition struct {
	Target        AnimationNodeIndex
	Condition     TransitionCondition
	// ParamName is the parameter checked for Threshold/Trigger conditions.
	ParamName     string
	// Threshold is compared against the named float parameter (Threshold condition).
	Threshold     float32
	BlendDuration float32 // seconds to cross-fade
}

// AnimationGraph is an asset defining a state machine of animation nodes.
// Nodes are stored in a flat slice; AnimationNodeIndex is an index into it.
// The state machine is acyclic for non-looping edges; looping is expressed
// via TransitionClipFinished edges back to self or an earlier node.
type AnimationGraph struct {
	Nodes      []AnimationNode
	Transitions [][]Transition // per-node outgoing transitions
	// Params holds float parameters driving blend/threshold conditions.
	Params map[string]float32
	// Triggers are one-shot boolean signals that reset after evaluation.
	Triggers map[string]bool
}
