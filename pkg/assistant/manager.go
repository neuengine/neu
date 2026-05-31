//go:build editor

package assistant

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Sentinel errors for assistant dispatch.
var (
	// ErrUnknownAgent: no agent registered under the given ID.
	ErrUnknownAgent = errors.New("assistant: unknown agent")
	// ErrCapabilityDenied: the method requires a capability the agent lacks (INV-2).
	ErrCapabilityDenied = errors.New("assistant: capability not granted")
	// ErrAgentUnavailable: transport error/timeout — editor degrades gracefully (INV-5).
	ErrAgentUnavailable = errors.New("assistant: agent unreachable or timed out")
)

// DefaultTimeout bounds every agent request (L1 §4.9).
const DefaultTimeout = 30 * time.Second

// RequestLogEntry records one dispatch for audit + undo grouping (INV-4).
type RequestLogEntry struct {
	Agent   AgentID
	Request RequestID
	Method  string
	OK      bool
}

// agentConn holds a connected agent's transport + granted capabilities.
type agentConn struct {
	conn Connection
	caps Capability
}

// AssistantManager owns connected agents, enforces the capability model, and
// records the request log. It is an editor resource registered by the AI
// assistant plugin.
type AssistantManager struct {
	mu      sync.Mutex
	agents  map[AgentID]*agentConn
	log     []RequestLogEntry
	timeout time.Duration
	seq     uint64
}

// NewAssistantManager returns a manager with the given per-request timeout
// (DefaultTimeout if <= 0).
func NewAssistantManager(timeout time.Duration) *AssistantManager {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &AssistantManager{agents: make(map[AgentID]*agentConn), timeout: timeout}
}

// RegisterAgent connects an agent with its granted capability set.
func (m *AssistantManager) RegisterAgent(id AgentID, conn Connection, caps Capability) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents[id] = &agentConn{conn: conn, caps: caps}
}

// nextRequestID allocates a unique, deterministic request ID for tagging.
func (m *AssistantManager) nextRequestID() RequestID {
	m.seq++
	return RequestID(fmt.Sprintf("req-%d", m.seq))
}

// Dispatch sends a method call to an agent and returns its response. It enforces
// the capability model (INV-2) and bounds the call with a timeout so a slow or
// dead agent never blocks the editor (INV-5). Every dispatch is recorded in the
// request log tagged with the agent + request ID (INV-4).
func (m *AssistantManager) Dispatch(ctx context.Context, id AgentID, method string, params map[string]any) (AgentMessage, error) {
	m.mu.Lock()
	ac, ok := m.agents[id]
	if !ok {
		m.mu.Unlock()
		return AgentMessage{}, ErrUnknownAgent
	}
	req := m.nextRequestID()
	// INV-2: the method's required capability must be granted.
	if need := RequiredCapability(method); !ac.caps.Has(need) {
		m.log = append(m.log, RequestLogEntry{Agent: id, Request: req, Method: method, OK: false})
		m.mu.Unlock()
		return AgentMessage{}, fmt.Errorf("%w: %s needs %s", ErrCapabilityDenied, method, need)
	}
	conn := ac.conn
	m.mu.Unlock()

	// INV-5: bound the request; a timeout/transport error degrades gracefully.
	cctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	msg := AgentMessage{ID: string(req), Type: MsgRequest, Method: method, Params: params}
	if err := conn.Send(cctx, msg); err != nil {
		m.record(id, req, method, false)
		return AgentMessage{}, fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	resp, err := conn.Receive(cctx)
	if err != nil {
		m.record(id, req, method, false)
		return AgentMessage{}, fmt.Errorf("%w: %v", ErrAgentUnavailable, err)
	}
	m.record(id, req, method, true)
	return resp, nil
}

func (m *AssistantManager) record(id AgentID, req RequestID, method string, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.log = append(m.log, RequestLogEntry{Agent: id, Request: req, Method: method, OK: ok})
}

// RequestLog returns a copy of the dispatch log.
func (m *AssistantManager) RequestLog() []RequestLogEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]RequestLogEntry(nil), m.log...)
}

// AgentCount returns the number of connected agents.
func (m *AssistantManager) AgentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.agents)
}

// --- Modification path (INV-1 / INV-4) ---

// Modification is a world edit an agent requested, tagged with its source so it
// flows through the Command pipeline as one undoable, auditable group (INV-1/4).
type Modification struct {
	Agent   AgentID
	Request RequestID
	Op      string         // "spawn" | "insert" | "set" | ...
	Args    map[string]any
}

// Modifier applies tagged modifications via the engine's Command pipeline. The
// engine supplies a CommandBuffer-backed implementation; tests use a recorder.
// This keeps the manager decoupled from the concrete command surface (INV-1).
type Modifier interface {
	Apply(mod Modification)
}

// ApplyModifications routes a request's edits through the modifier, tagging each
// with the agent + request ID. The whole slice is one undo group (INV-4).
func (m *AssistantManager) ApplyModifications(mod Modifier, agent AgentID, req RequestID, ops []Modification) {
	for i := range ops {
		ops[i].Agent = agent
		ops[i].Request = req
		mod.Apply(ops[i])
	}
}
