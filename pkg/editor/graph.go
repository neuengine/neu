package editor

// Visual-graph editor bridge (l2-visual-graph-editor-bridge §4.1–§4.3): the
// engine-side "door" the external editor repo drives to run a Blueprint-style
// graph UI. Interface + data only — concrete implementations live in the engine
// and are registered only when an editor is attached (multi-repo INV-3, INV-5).
//
// These types are deliberately self-contained (string IDs, plain DTOs, uint64
// entity IDs) so the editor repo links a stable contract without importing the
// engine's internal graph types — mirroring the opaque [DefinitionNode] pattern.

// Direction is the flow direction of a node pin.
type Direction uint8

const (
	Input Direction = iota
	Output
)

// PinDescriptor describes one pin of a node-palette entry.
type PinDescriptor struct {
	ID        string
	Name      string
	DataType  string
	Direction Direction
	Execution bool // true = execution pin, false = data pin
}

// NodeDescriptor is a node-palette entry surfaced to the editor.
type NodeDescriptor struct {
	TypeName    string
	DisplayName string
	Category    string
	Description string
	Pins        []PinDescriptor
}

// Connection is an editor-facing wire between two node pins.
type Connection struct {
	FromNode string
	FromPin  string
	ToNode   string
	ToPin    string
}

// ConnectionAction is the kind of edit in a [ConnectionChange].
type ConnectionAction uint8

const (
	ConnectionAdd ConnectionAction = iota
	ConnectionRemove
)

// ConnectionChange describes an editor connection edit (§4.1).
type ConnectionChange struct {
	Connection Connection
	Action     ConnectionAction
}

// ValidationResult reports whether an editor edit is acceptable (§4.1). It is
// advisory only — validation never mutates graph state.
type ValidationResult struct {
	Errors   []string
	Warnings []string
	Valid    bool
}

// NodeInspection is the runtime snapshot of a selected node (§4.1).
type NodeInspection struct {
	PinValues      map[string]any // current runtime value at each pin
	NodeID         string
	NodeType       string
	LastError      string
	ExecutionCount uint64
}

// GraphExecutionFrame is one step of a graph execution trace (§4.3). It is the
// editor-side inspection view (the wire form lives in pkg/protocol).
type GraphExecutionFrame struct {
	PinValues map[string]any
	NodeID    string
	NodeType  string
	Timestamp int64
	StepIndex uint32
}

// GraphEditorPlugin is the engine-side door for live graph editing (§4.1).
// Mutating callbacks return a [ValidationResult] *before* the edit is committed,
// so an invalid edit can be rejected without changing runtime state.
type GraphEditorPlugin interface {
	// OnGraphOpened prepares debugging state for the graph the editor opened.
	// The graph is identified by ID; its content is loaded by the engine or
	// pushed via the GraphLiveUpdate protocol message.
	OnGraphOpened(graphID string)
	// OnGraphClosed releases any debug state for the graph.
	OnGraphClosed(graphID string)
	// OnNodeSelected returns the current runtime values for the selected node.
	OnNodeSelected(graphID, nodeID string) NodeInspection
	// OnConnectionChanged validates an added/removed connection before commit.
	OnConnectionChanged(graphID string, change ConnectionChange) ValidationResult
	// OnPropertyChanged validates a node-property edit before commit.
	OnPropertyChanged(graphID, nodeID, property string, value any) ValidationResult
}

// NodeRegistryQuery powers the editor's node palette and connection assistance (§4.2).
type NodeRegistryQuery interface {
	// ListAllNodes returns every registered node type for the palette.
	ListAllNodes() []NodeDescriptor
	// SearchNodes filters the palette for the "add node" context menu.
	SearchNodes(query, category string) []NodeDescriptor
	// GetNodeDescriptor returns the descriptor for a type, or ok=false.
	GetNodeDescriptor(typeName string) (NodeDescriptor, bool)
	// GetCompatibleNodes returns nodes with a pin compatible with the dragged pin.
	GetCompatibleNodes(pinType string, dir Direction) []NodeDescriptor
	// GetTypeHierarchy returns the types assignable from typeName.
	GetTypeHierarchy(typeName string) []string
}

// GraphDebugger drives breakpoints and stepping for a running graph (§4.3).
// Entities are addressed by their raw uint64 ID to avoid an engine-type import.
type GraphDebugger interface {
	SetBreakpoint(graphID, nodeID string) error
	RemoveBreakpoint(graphID, nodeID string) error
	ListBreakpoints(graphID string) []string

	StepOver(graphID string) GraphExecutionFrame
	StepInto(graphID string) GraphExecutionFrame // enters a subgraph
	Continue(graphID string) error

	GetExecutionTrace(graphID string, maxFrames int) []GraphExecutionFrame
	GetVariableValues(graphID string, entityID uint64) map[string]any
	GetPinValue(graphID, nodeID, pinID string, entityID uint64) any
}
