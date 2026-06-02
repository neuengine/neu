package protocol

// Visual-graph debugging IPC messages (l2-visual-graph-editor-bridge §4.4).
// These extend the engine↔editor protocol with breakpoint, trace, runtime-error,
// and live-update events. Like every protocol type they are stdlib-only,
// self-contained wire DTOs (INV-4) — the engine maps its internal graph types
// onto these for transport.

const (
	KindGraphBreakpointHit  Kind = "GraphBreakpointHit"
	KindGraphExecutionTrace Kind = "GraphExecutionTrace"
	KindGraphRuntimeError   Kind = "GraphRuntimeError"
	KindGraphLiveUpdate     Kind = "GraphLiveUpdate"
)

// GraphChangeType classifies a [GraphLiveUpdate] edit pushed by the editor.
type GraphChangeType string

const (
	GraphNodeAdded         GraphChangeType = "NodeAdded"
	GraphNodeRemoved       GraphChangeType = "NodeRemoved"
	GraphConnectionAdded   GraphChangeType = "ConnectionAdded"
	GraphConnectionRemoved GraphChangeType = "ConnectionRemoved"
	GraphPropertyChanged   GraphChangeType = "PropertyChanged"
)

// GraphExecutionFrame is the wire form of one graph execution step. It is a
// flat, self-contained DTO (distinct from the editor-side inspection type) so
// pkg/protocol stays free of engine imports.
type GraphExecutionFrame struct {
	PinValues map[string]any `json:"pin_values"`
	NodeID    string         `json:"node_id"`
	NodeType  string         `json:"node_type"`
	Timestamp int64          `json:"timestamp"`
	StepIndex uint32         `json:"step_index"`
}

// GraphBreakpointHit (Engine → Editor): execution paused at a breakpoint.
type GraphBreakpointHit struct {
	Type     Kind                `json:"type"`
	GraphID  string              `json:"graph_id"`
	NodeID   string              `json:"node_id"`
	Frame    GraphExecutionFrame `json:"frame"`
	EntityID uint64              `json:"entity_id"`
}

// GraphExecutionTraceEvent (Engine → Editor): a real-time execution trace.
type GraphExecutionTraceEvent struct {
	Type     Kind                  `json:"type"`
	GraphID  string                `json:"graph_id"`
	Frames   []GraphExecutionFrame `json:"frames"`
	EntityID uint64                `json:"entity_id"`
}

// GraphRuntimeError (Engine → Editor): a graph hit a runtime error.
type GraphRuntimeError struct {
	PinValues map[string]any `json:"pin_values"`
	Type      Kind           `json:"type"`
	GraphID   string         `json:"graph_id"`
	NodeID    string         `json:"node_id"`
	Error     string         `json:"error"`
	EntityID  uint64         `json:"entity_id"`
}

// GraphLiveUpdate (Editor → Engine): a graph modification for immediate preview.
// Payload is change-specific (decoded by the receiver per ChangeType).
type GraphLiveUpdate struct {
	Payload    any             `json:"payload"`
	Type       Kind            `json:"type"`
	GraphID    string          `json:"graph_id"`
	ChangeType GraphChangeType `json:"change_type"`
}
