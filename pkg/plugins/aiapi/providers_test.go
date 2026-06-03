//go:build editor

package aiapi

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// --- Anthropic -------------------------------------------------------------

func TestAnthropicBuildRequest(t *testing.T) {
	t.Parallel()
	r := CanonicalRequest{
		Messages: []CanonicalMessage{
			TextMessage(RoleSystem, "be terse"),
			TextMessage(RoleUser, "hello"),
		},
		Parameters: map[string]any{"max_output_tokens": float64(256)},
	}
	w := buildAnthropicRequest("claude-x", r)
	if w.Model != "claude-x" || w.System != "be terse" {
		t.Errorf("build = %+v, want model claude-x + system hoisted", w)
	}
	if len(w.Messages) != 1 || w.Messages[0].Role != "user" || w.Messages[0].Content != "hello" {
		t.Errorf("messages = %+v, want one user turn (system hoisted out)", w.Messages)
	}
	if w.MaxTokens != 256 {
		t.Errorf("max_tokens = %d, want 256 from param", w.MaxTokens)
	}
	// Default max_tokens when unspecified.
	if d := buildAnthropicRequest("m", CanonicalRequest{}); d.MaxTokens != defaultMaxTokens {
		t.Errorf("default max_tokens = %d, want %d", d.MaxTokens, defaultMaxTokens)
	}
}

func TestAnthropicParseResponse(t *testing.T) {
	t.Parallel()
	body := `{"content":[{"type":"text","text":"hi "},{"type":"text","text":"there"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":2}}`
	resp, err := parseAnthropicResponse([]byte(body))
	if err != nil || resp.Text() != "hi there" || resp.Finish != "end_turn" || resp.Usage.OutputTokens != 2 {
		t.Errorf("parse = %+v, %v", resp, err)
	}
	if _, err := parseAnthropicResponse([]byte(`{bad`)); !asAPICode(err, CodeParse) {
		t.Error("malformed → CodeParse")
	}
	if _, err := parseAnthropicResponse([]byte(`{"content":[]}`)); !asAPICode(err, CodeParse) {
		t.Error("empty content → CodeParse")
	}
}

func TestAnthropicCompleteHTTP(t *testing.T) {
	t.Parallel()
	var gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("anthropic-version")
		if !strings.HasSuffix(r.URL.Path, "/messages") {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn","usage":{}}`))
	}))
	defer srv.Close()
	p := newAnthropic(ProviderConfig{Endpoint: srv.URL, Model: "m"})
	resp, err := p.Complete(context.Background(), CanonicalRequest{Messages: []CanonicalMessage{TextMessage(RoleUser, "ping")}})
	if err != nil || resp.Text() != "pong" {
		t.Errorf("Complete = %+v, %v", resp, err)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version header = %q, want %q", gotVersion, anthropicVersion)
	}
	// Stream falls back to a single final chunk.
	var got string
	if err := p.Stream(context.Background(), CanonicalRequest{}, func(c Chunk) error {
		got = c.Delta
		if !c.Final {
			t.Error("fallback stream chunk should be Final")
		}
		return nil
	}); err != nil || got != "pong" {
		t.Errorf("Stream = %q, %v", got, err)
	}
	if _, err := p.Embeddings(context.Background(), nil); err != ErrUnsupported {
		t.Errorf("Embeddings = %v, want ErrUnsupported", err)
	}
}

// --- Gemini ----------------------------------------------------------------

func TestGeminiBuildRequest(t *testing.T) {
	t.Parallel()
	r := CanonicalRequest{Messages: []CanonicalMessage{
		TextMessage(RoleSystem, "sys"),
		TextMessage(RoleUser, "u"),
		TextMessage(RoleAssistant, "a"),
	}}
	w := buildGeminiRequest(r)
	if w.SystemInstruction == nil || w.SystemInstruction.Parts[0].Text != "sys" {
		t.Errorf("systemInstruction = %+v, want sys", w.SystemInstruction)
	}
	if len(w.Contents) != 2 {
		t.Fatalf("contents = %d, want 2 (system hoisted)", len(w.Contents))
	}
	if w.Contents[0].Role != "user" || w.Contents[1].Role != "model" {
		t.Errorf("roles = %q,%q want user,model (assistant→model)", w.Contents[0].Role, w.Contents[1].Role)
	}
	// No system message → no systemInstruction.
	if d := buildGeminiRequest(CanonicalRequest{Messages: []CanonicalMessage{TextMessage(RoleUser, "x")}}); d.SystemInstruction != nil {
		t.Error("no system message ⇒ nil systemInstruction")
	}
}

func TestGeminiParseResponse(t *testing.T) {
	t.Parallel()
	body := `{"candidates":[{"content":{"parts":[{"text":"he"},{"text":"llo"}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":3}}`
	resp, err := parseGeminiResponse([]byte(body))
	if err != nil || resp.Text() != "hello" || resp.Finish != "STOP" || resp.Usage.InputTokens != 5 {
		t.Errorf("parse = %+v, %v", resp, err)
	}
	if _, err := parseGeminiResponse([]byte(`{bad`)); !asAPICode(err, CodeParse) {
		t.Error("malformed → CodeParse")
	}
	if _, err := parseGeminiResponse([]byte(`{"candidates":[]}`)); !asAPICode(err, CodeParse) {
		t.Error("no candidates → CodeParse")
	}
}

func TestGeminiCompleteHTTP(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, ":generateContent") {
			http.Error(w, "wrong path", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"pong"}]},"finishReason":"STOP"}],"usageMetadata":{}}`))
	}))
	defer srv.Close()
	p := newGemini(ProviderConfig{Endpoint: srv.URL, Model: "gemini-x"})
	resp, err := p.Complete(context.Background(), CanonicalRequest{Messages: []CanonicalMessage{TextMessage(RoleUser, "ping")}})
	if err != nil || resp.Text() != "pong" {
		t.Errorf("Complete = %+v, %v", resp, err)
	}
	var got string
	if err := p.Stream(context.Background(), CanonicalRequest{}, func(c Chunk) error { got = c.Delta; return nil }); err != nil || got != "pong" {
		t.Errorf("Stream fallback = %q, %v", got, err)
	}
	if _, err := p.Embeddings(context.Background(), nil); err != ErrUnsupported {
		t.Errorf("Embeddings = %v, want ErrUnsupported", err)
	}
}

func TestProviderNames(t *testing.T) {
	t.Parallel()
	if newAnthropic(ProviderConfig{}).Name() != "anthropic" {
		t.Error("anthropic Name")
	}
	if newGemini(ProviderConfig{}).Name() != "gemini" {
		t.Error("gemini Name")
	}
}

func TestAnthropicGeminiCompleteHTTPError(t *testing.T) {
	t.Parallel()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	if _, err := newAnthropic(ProviderConfig{Endpoint: bad.URL, Model: "m"}).Complete(context.Background(), CanonicalRequest{}); !asAPICode(err, CodeHTTP5xx) {
		t.Errorf("anthropic 500 = %v, want CodeHTTP5xx", err)
	}
	if _, err := newGemini(ProviderConfig{Endpoint: bad.URL, Model: "m"}).Complete(context.Background(), CanonicalRequest{}); !asAPICode(err, CodeHTTP5xx) {
		t.Errorf("gemini 500 = %v, want CodeHTTP5xx", err)
	}
}

func TestRunParity_WrapsError(t *testing.T) {
	t.Parallel()
	p := NewFakeProvider("x")
	p.Err = APIError{Code: CodeHTTP5xx, Message: "boom"}
	err := RunParity(context.Background(), p, []CanonicalRequest{{}})
	if err == nil || !strings.Contains(err.Error(), "request 0") {
		t.Errorf("RunParity error = %v, want wrapped 'request 0'", err)
	}
}

// --- Registry: all four providers selectable -------------------------------

func TestAllProvidersRegistered(t *testing.T) {
	t.Parallel()
	want := map[string]bool{"openai": false, "anthropic": false, "gemini": false, "local": false}
	for _, n := range RegisteredProviders() {
		if _, ok := want[n]; ok {
			want[n] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("provider %q not registered", name)
		}
	}
}

func TestLocalProviderIsOpenAICompatible(t *testing.T) {
	t.Parallel()
	// "local" must build an OpenAI-compatible provider (same /chat/completions path).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			http.Error(w, "not openai path", http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"local-pong"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()
	cfg := DefaultConfig()
	cfg.ActiveProvider = "local"
	cfg.Providers["local"] = ProviderConfig{Endpoint: srv.URL, Model: "llama"}
	p, err := Select(cfg)
	if err != nil {
		t.Fatalf("Select(local): %v", err)
	}
	resp, err := p.Complete(context.Background(), CanonicalRequest{Messages: []CanonicalMessage{TextMessage(RoleUser, "hi")}})
	if err != nil || resp.Text() != "local-pong" {
		t.Errorf("local Complete = %+v, %v", resp, err)
	}
}

// --- postJSON helper edge cases --------------------------------------------

func TestPostJSON_StatusAndBodyError(t *testing.T) {
	t.Parallel()
	// 401 → CodeUnauthorized.
	un := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer un.Close()
	if _, err := postJSON(context.Background(), http.DefaultClient, un.URL, []byte(`{}`), nil); !asAPICode(err, CodeUnauthorized) {
		t.Error("401 → CodeUnauthorized")
	}
	// Custom header forwarded.
	var gotHdr string
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHdr = r.Header.Get("X-Test")
		_, _ = io.WriteString(w, "{}")
	}))
	defer hs.Close()
	if _, err := postJSON(context.Background(), http.DefaultClient, hs.URL, []byte(`{}`), map[string]string{"X-Test": "v"}); err != nil {
		t.Fatalf("postJSON: %v", err)
	}
	if gotHdr != "v" {
		t.Errorf("header X-Test = %q, want v", gotHdr)
	}
}

// --- Parity harness (INV-7) ------------------------------------------------

func TestCheckParity_FakeProvider(t *testing.T) {
	t.Parallel()
	p := NewFakeProvider("deterministic answer")
	reqs := []CanonicalRequest{
		{Messages: []CanonicalMessage{TextMessage(RoleUser, "q1")}},
		{Messages: []CanonicalMessage{TextMessage(RoleSystem, "sys"), TextMessage(RoleUser, "q2")}, Parameters: map[string]any{"temperature": 0.7}},
	}
	if err := RunParity(context.Background(), p, reqs); err != nil {
		t.Errorf("RunParity (INV-7) = %v, want nil", err)
	}
}

func TestCheckParity_PropagatesProviderError(t *testing.T) {
	t.Parallel()
	p := NewFakeProvider("x")
	p.Err = APIError{Code: CodeHTTP5xx, Message: "boom"}
	if err := CheckParity(context.Background(), p, CanonicalRequest{}); err == nil {
		t.Error("provider error should propagate through CheckParity")
	}
}

func TestCompleteOverWire_RoundTripsCanonical(t *testing.T) {
	t.Parallel()
	// A response whose fields all round-trip cleanly through JSON.
	p := NewFakeProvider("round-trip")
	got, err := completeOverWire(context.Background(), p, CanonicalRequest{})
	if err != nil {
		t.Fatalf("completeOverWire: %v", err)
	}
	// Confirm the wire form is decodable + equal to a direct marshal.
	direct, _ := p.Complete(context.Background(), CanonicalRequest{})
	db, _ := json.Marshal(direct)
	gb, _ := json.Marshal(got)
	if string(db) != string(gb) {
		t.Errorf("wire response %s != direct %s", gb, db)
	}
}
