//go:build editor

package assistant

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// echoServer returns an httptest.Server that decodes the request AgentMessage
// and replies with a response echoing its ID plus a fixed result.
func echoServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "want POST", http.StatusMethodNotAllowed)
			return
		}
		body, _ := io.ReadAll(r.Body)
		var req AgentMessage
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		resp := AgentMessage{ID: req.ID, Type: MsgResponse, Result: "pong:" + req.Method}
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestHTTPTransport_RoundTrip(t *testing.T) {
	t.Parallel()
	srv := echoServer(t)
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, err := tr.Connect(srv.URL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer tr.Close()

	if !conn.IsAlive() {
		t.Fatal("new connection should be alive")
	}

	ctx := context.Background()
	if err := conn.Send(ctx, AgentMessage{ID: "7", Type: MsgRequest, Method: "chat"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	resp, err := conn.Receive(ctx)
	if err != nil {
		t.Fatalf("Receive: %v", err)
	}
	if resp.ID != "7" || resp.Result != "pong:chat" {
		t.Errorf("response = %+v, want ID=7 Result=pong:chat", resp)
	}
}

func TestHTTPTransport_DispatchIntegration(t *testing.T) {
	t.Parallel()
	srv := echoServer(t)
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, err := tr.Connect(srv.URL)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	// The HTTP connection plugs into the manager exactly like stdio/mem.
	m := NewAssistantManager(time.Second)
	m.RegisterAgent("http-agent", conn, ReadTypeRegistry)

	resp, err := m.Dispatch(context.Background(), "http-agent", MethodChat, map[string]any{"prompt": "hi"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if resp.Result != "pong:chat" {
		t.Errorf("dispatch result = %v, want pong:chat", resp.Result)
	}
}

func TestHTTPTransport_EmptyEndpoint(t *testing.T) {
	t.Parallel()
	tr := &HTTPTransport{}
	if _, err := tr.Connect(""); err == nil {
		t.Fatal("empty endpoint should error")
	}
}

func TestHTTPTransport_DefaultClient(t *testing.T) {
	t.Parallel()
	// Connect with a nil Client must fall back to http.DefaultClient (no panic).
	tr := &HTTPTransport{}
	conn, err := tr.Connect("http://example.invalid")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if conn == nil {
		t.Fatal("connection should not be nil")
	}
}

func TestHTTPConnection_Non2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, _ := tr.Connect(srv.URL)
	if err := conn.Send(context.Background(), AgentMessage{ID: "1"}); err == nil {
		t.Fatal("non-2xx status should error")
	}
}

func TestHTTPConnection_BadResponseJSON(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, _ := tr.Connect(srv.URL)
	if err := conn.Send(context.Background(), AgentMessage{ID: "1"}); err == nil {
		t.Fatal("malformed response JSON should error")
	}
}

func TestHTTPConnection_SendAfterClose(t *testing.T) {
	t.Parallel()
	srv := echoServer(t)
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, _ := tr.Connect(srv.URL)
	conn.(*HTTPConnection).Close()
	if conn.IsAlive() {
		t.Error("closed connection should not be alive")
	}
	if err := conn.Send(context.Background(), AgentMessage{}); err != ErrConnClosed {
		t.Errorf("Send after close = %v, want ErrConnClosed", err)
	}
}

func TestHTTPConnection_ReceiveWithoutSend(t *testing.T) {
	t.Parallel()
	srv := echoServer(t)
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, _ := tr.Connect(srv.URL)
	if _, err := conn.Receive(context.Background()); err != ErrConnClosed {
		t.Errorf("Receive without Send = %v, want ErrConnClosed", err)
	}
}

func TestHTTPConnection_ReceiveSingleShot(t *testing.T) {
	t.Parallel()
	srv := echoServer(t)
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, _ := tr.Connect(srv.URL)
	ctx := context.Background()

	_ = conn.Send(ctx, AgentMessage{ID: "1", Method: "chat"})
	if _, err := conn.Receive(ctx); err != nil {
		t.Fatalf("first Receive: %v", err)
	}
	// Second Receive without a new Send drains nothing.
	if _, err := conn.Receive(ctx); err != ErrConnClosed {
		t.Errorf("second Receive = %v, want ErrConnClosed (single-shot)", err)
	}
}

func TestHTTPConnection_CancelledContext(t *testing.T) {
	t.Parallel()
	srv := echoServer(t)
	defer srv.Close()

	tr := &HTTPTransport{Client: srv.Client()}
	conn, _ := tr.Connect(srv.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := conn.Send(ctx, AgentMessage{}); err == nil {
		t.Error("cancelled context Send should error")
	}
	if _, err := conn.Receive(ctx); err == nil {
		t.Error("cancelled context Receive should error")
	}
}

func TestHTTPConnection_TransportError(t *testing.T) {
	t.Parallel()
	// Point at a server we immediately close → client.Do fails.
	srv := echoServer(t)
	url := srv.URL
	client := srv.Client()
	srv.Close()

	tr := &HTTPTransport{Client: client}
	conn, _ := tr.Connect(url)
	if err := conn.Send(context.Background(), AgentMessage{ID: "1"}); err == nil {
		t.Error("transport error should be reported")
	}
}
