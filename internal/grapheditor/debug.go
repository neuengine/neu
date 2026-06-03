// Package grapheditor provides the engine-side concrete implementations of the
// pkg/editor visual-graph contract (l2-visual-graph-editor-bridge §4.1–§4.4).
// They map the headless pkg/visualgraph runtime (registry, model, validation)
// onto the editor's self-contained DTOs and are registered only when an editor
// is attached (multi-repo INV-3/INV-5). The package imports pkg/editor and
// pkg/visualgraph but never the reverse — pkg/editor stays contract-only.
package grapheditor

import (
	"sort"
	"sync"

	"github.com/neuengine/neu/pkg/editor"
)

// DefaultMaxTrace bounds the per-graph execution-trace ring so a long-running
// graph never grows debug memory without bound.
const DefaultMaxTrace = 256

// TraceEvent is one recorded execution step or runtime error, tagged with its
// originating graph + entity and whether the node carried an active breakpoint.
// The graphDebugSyncSystem (T-6P04·rem.2) drains these and maps them onto the
// pkg/protocol graph IPC messages; keeping that mapping out of the store leaves
// this package free of any protocol import.
type TraceEvent struct {
	Frame      editor.GraphExecutionFrame
	GraphID    string
	Err        string // non-empty → a runtime error occurred at Frame.NodeID
	EntityID   uint64
	Breakpoint bool // the node had an active breakpoint when this frame was recorded
}

// graphDebug is the mutable debug state for one open graph.
type graphDebug struct {
	breakpoints map[string]struct{}
	execCount   map[string]uint64
	lastErr     map[string]string
	latest      map[string]editor.GraphExecutionFrame // node ID → most recent frame
	trace       []editor.GraphExecutionFrame
	cursor      int // replay position for StepOver/StepInto
}

func newGraphDebug() *graphDebug {
	return &graphDebug{
		breakpoints: make(map[string]struct{}),
		execCount:   make(map[string]uint64),
		lastErr:     make(map[string]string),
		latest:      make(map[string]editor.GraphExecutionFrame),
	}
}

// DebugStore is the shared debug state behind the EditorPlugin and Debugger. It
// owns per-graph breakpoints, execution traces, and runtime stats, and queues
// TraceEvents for the sync system to forward to the editor. All methods are safe
// for concurrent use: the interpreter records from the schedule while the editor
// queries from the IPC handler.
type DebugStore struct {
	graphs   map[string]*graphDebug
	pending  []TraceEvent
	maxTrace int
	mu       sync.RWMutex
}

// NewDebugStore returns an empty store with the default trace bound.
func NewDebugStore() *DebugStore {
	return &DebugStore{graphs: make(map[string]*graphDebug), maxTrace: DefaultMaxTrace}
}

// Open registers a graph for debugging. Idempotent: re-opening preserves state.
func (s *DebugStore) Open(graphID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.graphs[graphID]; !ok {
		s.graphs[graphID] = newGraphDebug()
	}
}

// Close drops all debug state for a graph.
func (s *DebugStore) Close(graphID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.graphs, graphID)
}

// IsOpen reports whether a graph is currently open for debugging.
func (s *DebugStore) IsOpen(graphID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.graphs[graphID]
	return ok
}

// SetBreakpoint marks a node as a breakpoint. Returns false if the graph is not
// open (the caller turns that into an error).
func (s *DebugStore) SetBreakpoint(graphID, nodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return false
	}
	g.breakpoints[nodeID] = struct{}{}
	return true
}

// RemoveBreakpoint clears a node breakpoint. Returns false if the graph is not
// open or no breakpoint was set there.
func (s *DebugStore) RemoveBreakpoint(graphID, nodeID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return false
	}
	if _, set := g.breakpoints[nodeID]; !set {
		return false
	}
	delete(g.breakpoints, nodeID)
	return true
}

// ListBreakpoints returns the breakpointed node IDs, sorted (deterministic).
func (s *DebugStore) ListBreakpoints(graphID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return nil
	}
	out := make([]string, 0, len(g.breakpoints))
	for id := range g.breakpoints {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// RecordFrame appends an execution frame to a graph's trace, bumps the node's
// execution count, remembers it as the node's latest frame, and queues a
// TraceEvent (flagged when the node is breakpointed). No-op if the graph is not
// open — the interpreter runs identically with or without an editor attached.
func (s *DebugStore) RecordFrame(graphID string, entityID uint64, frame editor.GraphExecutionFrame) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return
	}
	g.execCount[frame.NodeID]++
	g.latest[frame.NodeID] = frame
	g.trace = append(g.trace, frame)
	if len(g.trace) > s.maxTrace {
		g.trace = g.trace[len(g.trace)-s.maxTrace:]
	}
	_, bp := g.breakpoints[frame.NodeID]
	s.pending = append(s.pending, TraceEvent{
		GraphID:    graphID,
		EntityID:   entityID,
		Frame:      frame,
		Breakpoint: bp,
	})
}

// RecordError records a node's runtime error and queues an error TraceEvent. The
// frame carries the node's most recent pin values when available. No-op if the
// graph is not open.
func (s *DebugStore) RecordError(graphID string, entityID uint64, nodeID, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return
	}
	g.lastErr[nodeID] = errMsg
	frame := g.latest[nodeID]
	frame.NodeID = nodeID
	s.pending = append(s.pending, TraceEvent{
		GraphID:  graphID,
		EntityID: entityID,
		Frame:    frame,
		Err:      errMsg,
	})
}

// DrainEvents returns and clears the queued TraceEvents. The sync system calls
// this once per frame and maps the result onto pkg/protocol messages.
func (s *DebugStore) DrainEvents() []TraceEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.pending) == 0 {
		return nil
	}
	out := s.pending
	s.pending = nil
	return out
}

// Trace returns up to maxFrames most-recent frames for a graph (all when
// maxFrames <= 0). The result is a copy, safe to read without the lock.
func (s *DebugStore) Trace(graphID string, maxFrames int) []editor.GraphExecutionFrame {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return nil
	}
	t := g.trace
	if maxFrames > 0 && len(t) > maxFrames {
		t = t[len(t)-maxFrames:]
	}
	out := make([]editor.GraphExecutionFrame, len(t))
	copy(out, t)
	return out
}

// Inspect returns a node's runtime stats: execution count, last error, and the
// pin values from its most recent frame (nil if it has not executed).
func (s *DebugStore) Inspect(graphID, nodeID string) (count uint64, lastErr string, pins map[string]any) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	g, ok := s.graphs[graphID]
	if !ok {
		return 0, "", nil
	}
	count = g.execCount[nodeID]
	lastErr = g.lastErr[nodeID]
	if f, ok := g.latest[nodeID]; ok {
		pins = f.PinValues
	}
	return count, lastErr, pins
}

// StepNext advances the replay cursor and returns the next recorded frame, with
// ok=false once the cursor reaches the end of the trace.
func (s *DebugStore) StepNext(graphID string) (editor.GraphExecutionFrame, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.graphs[graphID]
	if !ok || g.cursor >= len(g.trace) {
		return editor.GraphExecutionFrame{}, false
	}
	f := g.trace[g.cursor]
	g.cursor++
	return f, true
}

// ResetCursor rewinds the replay cursor to the start of the trace.
func (s *DebugStore) ResetCursor(graphID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if g, ok := s.graphs[graphID]; ok {
		g.cursor = 0
	}
}
