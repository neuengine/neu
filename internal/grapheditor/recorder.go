package grapheditor

import (
	"time"

	"github.com/neuengine/neu/pkg/editor"
	vg "github.com/neuengine/neu/pkg/visualgraph"
)

// Recorder adapts a DebugStore to visualgraph.TraceRecorder for one running graph
// instance (a graph + the entity executing it). The interpreter calls RecordStep
// per executed node; this maps the neutral visualgraph frame onto the editor
// frame (stamping the wall-clock time) and records it into the shared store.
// Holding the mapping here keeps pkg/visualgraph free of any editor type.
type Recorder struct {
	store    *DebugStore
	now      func() int64
	graphID  string
	entityID uint64
}

// NewRecorder builds a recorder bound to a graph + entity, writing into store.
func NewRecorder(store *DebugStore, graphID string, entityID uint64) *Recorder {
	return &Recorder{store: store, now: nowNano, graphID: graphID, entityID: entityID}
}

func nowNano() int64 { return time.Now().UnixNano() }

// RecordStep maps a visualgraph execution frame to the editor frame and records
// it. Implements visualgraph.TraceRecorder.
func (r *Recorder) RecordStep(f vg.ExecutionFrame) {
	r.store.RecordFrame(r.graphID, r.entityID, editor.GraphExecutionFrame{
		PinValues: f.PinValues,
		NodeID:    f.NodeID,
		NodeType:  f.NodeType,
		Timestamp: r.now(),
		StepIndex: f.StepIndex,
	})
}

// RecordError records a runtime error against the recorder's graph + entity, so
// a failed Run surfaces to the editor as a GraphRuntimeError via the sync system.
func (r *Recorder) RecordError(nodeID, msg string) {
	r.store.RecordError(r.graphID, r.entityID, nodeID, msg)
}

var _ vg.TraceRecorder = (*Recorder)(nil)
