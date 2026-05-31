//go:build editor

package assistant

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"
)

// ErrConnClosed is returned by a closed connection's Send/Receive.
var ErrConnClosed = errors.New("assistant: connection closed")

// Transport opens connections to an agent endpoint (L1 §4.2).
type Transport interface {
	Connect(endpoint string) (Connection, error)
	Close()
}

// Connection is a live, message-oriented link to one agent. All methods honour
// the context so a slow/dead agent never blocks the editor (INV-5).
type Connection interface {
	Send(ctx context.Context, m AgentMessage) error
	Receive(ctx context.Context) (AgentMessage, error)
	IsAlive() bool
}

// --- stdio transport: agent runs as a child process, newline-delimited JSON ---

// StdioConnection speaks newline-delimited JSON over a reader/writer pair (the
// child process's stdout/stdin). Closing it stops further I/O.
type StdioConnection struct {
	mu     sync.Mutex
	w      io.Writer
	r      *bufio.Reader
	closed bool
}

// NewStdioConnection wraps a writer (to the agent's stdin) and reader (from its
// stdout).
func NewStdioConnection(w io.Writer, r io.Reader) *StdioConnection {
	return &StdioConnection{w: w, r: bufio.NewReader(r)}
}

// Send writes one newline-framed message, respecting ctx cancellation.
func (c *StdioConnection) Send(ctx context.Context, m AgentMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return ErrConnClosed
	}
	b, err := Encode(m)
	if err != nil {
		return err
	}
	_, err = c.w.Write(b)
	return err
}

// Receive reads the next newline-framed message. Cancellation is checked before
// the (potentially blocking) read; a full async reader is a follow-up.
func (c *StdioConnection) Receive(ctx context.Context) (AgentMessage, error) {
	if err := ctx.Err(); err != nil {
		return AgentMessage{}, err
	}
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		return AgentMessage{}, err
	}
	return Decode(line)
}

// IsAlive reports whether the connection is open.
func (c *StdioConnection) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return !c.closed
}

// Close marks the connection closed.
func (c *StdioConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
}

// --- in-memory connection: deterministic responder for tests ---

// MemConnection is an in-process Connection backed by a response function. It
// makes manager dispatch, capability gating, and timeout behaviour testable
// without spawning a process (mirrors the audio/window headless pattern).
type MemConnection struct {
	// Respond produces the response for a sent request. A nil Respond echoes an
	// empty result. Return ctx.Err() honouring to simulate a slow agent.
	Respond func(ctx context.Context, req AgentMessage) (AgentMessage, error)
	alive   bool
	last    AgentMessage
	hasLast bool
}

// NewMemConnection returns a live in-memory connection.
func NewMemConnection(respond func(context.Context, AgentMessage) (AgentMessage, error)) *MemConnection {
	return &MemConnection{Respond: respond, alive: true}
}

// Send records the request and (lazily) computes the response for Receive.
func (c *MemConnection) Send(ctx context.Context, m AgentMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !c.alive {
		return ErrConnClosed
	}
	c.last = m
	c.hasLast = true
	return nil
}

// Receive returns the response for the last sent request, honouring ctx.
func (c *MemConnection) Receive(ctx context.Context) (AgentMessage, error) {
	if !c.hasLast {
		return AgentMessage{}, ErrConnClosed
	}
	if c.Respond == nil {
		return AgentMessage{ID: c.last.ID, Type: MsgResponse}, nil
	}
	return c.Respond(ctx, c.last)
}

// IsAlive reports liveness.
func (c *MemConnection) IsAlive() bool { return c.alive }

// Close marks the connection dead.
func (c *MemConnection) Close() { c.alive = false }

var _ Connection = (*StdioConnection)(nil)
var _ Connection = (*MemConnection)(nil)
