//go:build editor

package assistant

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// DefaultHTTPMaxResponseBytes caps an agent's HTTP response body so a hostile or
// broken backend can't exhaust editor memory (defence in depth alongside the
// dispatch timeout, INV-5).
const DefaultHTTPMaxResponseBytes = 8 << 20 // 8 MiB

// HTTPTransport connects to a stateless HTTP agent endpoint. Each request is one
// POST whose JSON body is the AgentMessage and whose JSON response body is the
// reply (L1 §4.2 "stateless request/response"). It is stdlib-only (net/http),
// so it needs no third-party dependency (C-003).
type HTTPTransport struct {
	// Client is the HTTP client used for requests. If nil, http.DefaultClient is
	// used. Inject a custom client (e.g. httptest) for tests or to tune timeouts.
	Client *http.Client
	// MaxResponseBytes caps the response body size; 0 uses DefaultHTTPMaxResponseBytes.
	MaxResponseBytes int64
}

// Connect returns a Connection bound to endpoint. No network call happens here —
// HTTP is stateless, so the round-trip occurs on each Send (L1 §4.2). An empty
// endpoint is rejected up front.
func (t *HTTPTransport) Connect(endpoint string) (Connection, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("assistant: http transport needs a non-empty endpoint")
	}
	client := t.Client
	if client == nil {
		client = http.DefaultClient
	}
	max := t.MaxResponseBytes
	if max <= 0 {
		max = DefaultHTTPMaxResponseBytes
	}
	return &HTTPConnection{endpoint: endpoint, client: client, maxBytes: max, alive: true}, nil
}

// Close releases transport-level resources. The stdlib client needs no teardown,
// so this is a no-op kept for interface symmetry.
func (t *HTTPTransport) Close() {}

// HTTPConnection is a stateless request/response link to an HTTP agent. Because
// HTTP couples a request with its response in one round-trip, Send performs the
// POST and buffers the decoded reply; Receive returns that buffered reply. This
// maps the coupled HTTP exchange onto the Send-then-Receive Connection contract
// (the same buffering MemConnection uses).
type HTTPConnection struct {
	endpoint string
	client   *http.Client
	maxBytes int64

	mu      sync.Mutex
	resp    AgentMessage
	hasResp bool
	alive   bool
}

// Send POSTs m as JSON to the endpoint and buffers the decoded response for the
// next Receive. The request honours ctx (cancellation/timeout, INV-5). A non-2xx
// status or a transport error is returned and leaves no buffered response.
func (c *HTTPConnection) Send(ctx context.Context, m AgentMessage) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	alive := c.alive
	c.mu.Unlock()
	if !alive {
		return ErrConnClosed
	}

	body, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("assistant: http marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("assistant: http build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("assistant: http do: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("assistant: http status %d", httpResp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(httpResp.Body, c.maxBytes))
	if err != nil {
		return fmt.Errorf("assistant: http read response: %w", err)
	}

	var resp AgentMessage
	if err := json.Unmarshal(data, &resp); err != nil {
		return fmt.Errorf("assistant: http decode response: %w", err)
	}

	c.mu.Lock()
	c.resp = resp
	c.hasResp = true
	c.mu.Unlock()
	return nil
}

// Receive returns the response buffered by the preceding Send. Calling Receive
// without a prior successful Send returns ErrConnClosed (there is nothing to
// return). The buffered response is consumed (single-shot per Send).
func (c *HTTPConnection) Receive(ctx context.Context) (AgentMessage, error) {
	if err := ctx.Err(); err != nil {
		return AgentMessage{}, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.hasResp {
		return AgentMessage{}, ErrConnClosed
	}
	c.hasResp = false
	return c.resp, nil
}

// IsAlive reports whether the connection is open.
func (c *HTTPConnection) IsAlive() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.alive
}

// Close marks the connection closed; subsequent Send returns ErrConnClosed.
func (c *HTTPConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.alive = false
}

var (
	_ Transport  = (*HTTPTransport)(nil)
	_ Connection = (*HTTPConnection)(nil)
)
