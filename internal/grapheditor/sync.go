package grapheditor

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/protocol"
)

// GraphEventSink receives graph IPC messages bound for the editor. The app wires
// it to the editor transport (pkg/protocol over a Connection); tests capture the
// messages. A nil sink drops events (the queue is still drained so it can't grow).
type GraphEventSink func(msg any)

// SyncPlugin installs the graph-debug sync system. Each PostUpdate it drains the
// DebugStore's queued TraceEvents and forwards them to the editor as pkg/protocol
// graph IPC messages (Engine → Editor: breakpoint hits, execution traces, runtime
// errors). It is opt-in / editor-attached: the app adds it only when an editor
// connection exists, so a headless build emits nothing. (GraphLiveUpdate is the
// inbound Editor → Engine direction and is handled by the IPC receiver, not here.)
type SyncPlugin struct {
	// Store is shared with the EditorPlugin/Debugger; a nil Store gets a fresh one.
	Store *DebugStore
	// Sink forwards encoded messages; nil drops them after draining.
	Sink GraphEventSink
}

// Build implements appface.Plugin.
func (p SyncPlugin) Build(app appface.Builder) {
	store := p.Store
	if store == nil {
		store = NewDebugStore()
	}
	app.SetResource(store)

	sink := p.Sink
	app.AddSystem(appface.PostUpdate, scheduler.NewFuncSystem("grapheditor.DebugSync", func(_ *world.World) {
		syncDebugEvents(store, sink)
	}))
}

// syncDebugEvents drains the store's queued events and forwards each to the sink.
// The drain happens unconditionally (so a nil sink can't let the queue grow); the
// mapping + send happen only when a sink is attached.
func syncDebugEvents(store *DebugStore, sink GraphEventSink) {
	events := store.DrainEvents()
	if sink == nil {
		return
	}
	for _, ev := range events {
		sink(toProtocol(ev))
	}
}

// toProtocol maps a TraceEvent onto the matching Engine → Editor graph IPC
// message: a runtime error → GraphRuntimeError, a breakpointed step →
// GraphBreakpointHit, an ordinary step → a single-frame GraphExecutionTraceEvent.
func toProtocol(ev TraceEvent) any {
	frame := protocol.GraphExecutionFrame{
		PinValues: ev.Frame.PinValues,
		NodeID:    ev.Frame.NodeID,
		NodeType:  ev.Frame.NodeType,
		Timestamp: ev.Frame.Timestamp,
		StepIndex: ev.Frame.StepIndex,
	}
	switch {
	case ev.Err != "":
		return protocol.GraphRuntimeError{
			PinValues: ev.Frame.PinValues,
			Type:      protocol.KindGraphRuntimeError,
			GraphID:   ev.GraphID,
			NodeID:    ev.Frame.NodeID,
			Error:     ev.Err,
			EntityID:  ev.EntityID,
		}
	case ev.Breakpoint:
		return protocol.GraphBreakpointHit{
			Type:     protocol.KindGraphBreakpointHit,
			GraphID:  ev.GraphID,
			NodeID:   ev.Frame.NodeID,
			Frame:    frame,
			EntityID: ev.EntityID,
		}
	default:
		return protocol.GraphExecutionTraceEvent{
			Type:     protocol.KindGraphExecutionTrace,
			GraphID:  ev.GraphID,
			Frames:   []protocol.GraphExecutionFrame{frame},
			EntityID: ev.EntityID,
		}
	}
}

var _ appface.Plugin = SyncPlugin{}
