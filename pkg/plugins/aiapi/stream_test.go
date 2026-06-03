//go:build editor

package aiapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// collectStream runs decodeSSE over body and returns the deltas + final flag.
func collectStream(t *testing.T, ctx context.Context, body string) (deltas []string, finalSeen bool, err error) {
	t.Helper()
	err = decodeSSE(ctx, strings.NewReader(body), func(c Chunk) error {
		deltas = append(deltas, c.Delta)
		if c.Final {
			finalSeen = true
		}
		return nil
	})
	return deltas, finalSeen, err
}

func TestDecodeSSE_MultiChunk(t *testing.T) {
	t.Parallel()
	body := `data: {"choices":[{"delta":{"content":"Hel"},"finish_reason":null}]}

data: {"choices":[{"delta":{"content":"lo"},"finish_reason":null}]}

data: {"choices":[{"delta":{},"finish_reason":"stop"}]}

data: [DONE]
`
	deltas, final, err := collectStream(t, context.Background(), body)
	if err != nil {
		t.Fatalf("decodeSSE: %v", err)
	}
	if got := strings.Join(deltas, ""); got != "Hello" {
		t.Errorf("accumulated = %q, want Hello", got)
	}
	if !final {
		t.Error("final chunk (finish_reason set) should be delivered with Final=true")
	}
}

func TestDecodeSSE_DoneTerminator(t *testing.T) {
	t.Parallel()
	// [DONE] before any finish_reason should still stop cleanly.
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n\ndata: [DONE]\n"
	deltas, _, err := collectStream(t, context.Background(), body)
	if err != nil {
		t.Fatalf("decodeSSE: %v", err)
	}
	if len(deltas) != 1 || deltas[0] != "x" {
		t.Errorf("deltas = %v, want [x]", deltas)
	}
}

func TestDecodeSSE_SkipsKeepaliveAndComments(t *testing.T) {
	t.Parallel()
	body := ": keepalive comment\n" +
		"event: ping\n" +
		"data: {\"choices\":[{\"delta\":{}}]}\n" + // role-only/empty delta → skipped
		"data: {\"choices\":[{\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n"
	deltas, final, err := collectStream(t, context.Background(), body)
	if err != nil {
		t.Fatalf("decodeSSE: %v", err)
	}
	if len(deltas) != 1 || deltas[0] != "ok" || !final {
		t.Errorf("deltas = %v final = %v, want [ok] true", deltas, final)
	}
}

func TestDecodeSSE_MalformedChunk(t *testing.T) {
	t.Parallel()
	body := "data: {not json}\n"
	_, _, err := collectStream(t, context.Background(), body)
	if !asAPICode(err, CodeParse) {
		t.Errorf("malformed chunk = %v, want CodeParse", err)
	}
}

func TestDecodeSSE_ContextCancelled(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := "data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"
	_, _, err := collectStream(t, ctx, body)
	if !asAPICode(err, CodeCancelled) {
		t.Errorf("cancelled stream = %v, want CodeCancelled (INV-6)", err)
	}
}

func TestDecodeSSE_SinkError(t *testing.T) {
	t.Parallel()
	boom := errors.New("sink boom")
	err := decodeSSE(context.Background(),
		strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n"),
		func(Chunk) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("sink error = %v, want propagated boom", err)
	}
}

func TestOpenAIProviderStream_HTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"pi\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\"ng\"},\"finish_reason\":\"stop\"}]}\n\n" +
			"data: [DONE]\n"))
	}))
	defer srv.Close()

	p := newOpenAI(ProviderConfig{Endpoint: srv.URL, Model: "m"})
	var got strings.Builder
	err := p.Stream(context.Background(), CanonicalRequest{Messages: []CanonicalMessage{TextMessage(RoleUser, "ping")}}, func(c Chunk) error {
		got.WriteString(c.Delta)
		return nil
	})
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	if got.String() != "ping" {
		t.Errorf("streamed text = %q, want ping", got.String())
	}
}

// errReader returns a non-EOF read error after an optional prefix, to exercise
// the scanner-error path of decodeSSE.
type errReader struct {
	prefix string
	done   bool
}

func (e *errReader) Read(p []byte) (int, error) {
	if !e.done && e.prefix != "" {
		n := copy(p, e.prefix)
		e.done = true
		return n, nil
	}
	return 0, errors.New("read failure")
}

func TestDecodeSSE_ScanError(t *testing.T) {
	t.Parallel()
	// A reader that yields a partial line then errors → scanner reports the error.
	err := decodeSSE(context.Background(), &errReader{prefix: "data: partial"}, func(Chunk) error { return nil })
	if !asAPICode(err, CodeParse) {
		t.Errorf("scan error = %v, want CodeParse", err)
	}
}

func TestOpenAIProviderStream_TransportError(t *testing.T) {
	t.Parallel()
	// Spin a server, capture its client+URL, then close it → client.Do fails.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	client := srv.Client()
	srv.Close()
	p := &openAIProvider{cfg: ProviderConfig{Endpoint: url, Model: "m"}, client: client}
	if err := p.Stream(context.Background(), CanonicalRequest{}, func(Chunk) error { return nil }); err == nil {
		t.Error("transport error should be reported")
	}
}

func TestOpenAIProviderStream_HTTPError(t *testing.T) {
	t.Parallel()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	p := newOpenAI(ProviderConfig{Endpoint: bad.URL, Model: "m"})
	err := p.Stream(context.Background(), CanonicalRequest{}, func(Chunk) error { return nil })
	if !asAPICode(err, CodeHTTP5xx) {
		t.Errorf("500 stream = %v, want CodeHTTP5xx", err)
	}
}
