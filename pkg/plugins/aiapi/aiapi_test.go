//go:build editor

package aiapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterRPM(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(2, 0) // 2 req/min, tokens disabled
	frozen := time.Now()
	rl.now = func() time.Time { return frozen }

	if ok, _ := rl.Allow(0); !ok {
		t.Fatal("first request should pass")
	}
	if ok, _ := rl.Allow(0); !ok {
		t.Fatal("second request should pass")
	}
	ok, retry := rl.Allow(0) // third exhausts the bucket
	if ok || retry < 1 {
		t.Errorf("third request = ok %v retry %d, want denied with retry-after (INV-8)", ok, retry)
	}
	// Advance 60s → bucket refills.
	rl.now = func() time.Time { return frozen.Add(60 * time.Second) }
	if ok, _ := rl.Allow(0); !ok {
		t.Error("after 60s refill the request should pass")
	}
}

func TestRateLimiterTPM(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(0, 100) // tokens only
	frozen := time.Now()
	rl.now = func() time.Time { return frozen }
	if ok, _ := rl.Allow(80); !ok {
		t.Fatal("80 tokens within 100 should pass")
	}
	if ok, retry := rl.Allow(80); ok || retry < 1 {
		t.Errorf("second 80-token request should be denied (only 20 left), got ok %v", ok)
	}
}

func TestMapHTTPStatus(t *testing.T) {
	t.Parallel()
	cases := map[int]struct {
		code  Code
		isErr bool
	}{
		200: {0, false},
		401: {CodeUnauthorized, true},
		429: {CodeRateLimited, true},
		404: {CodeHTTP4xx, true},
		503: {CodeHTTP5xx, true},
	}
	for status, want := range cases {
		code, isErr := MapHTTPStatus(status)
		if isErr != want.isErr || (isErr && code != want.code) {
			t.Errorf("MapHTTPStatus(%d) = %d,%v want %d,%v", status, code, isErr, want.code, want.isErr)
		}
	}
}

func TestOpenAIBuildParse(t *testing.T) {
	t.Parallel()
	req := CanonicalRequest{
		Messages:   []CanonicalMessage{TextMessage(RoleUser, "hello")},
		Parameters: map[string]any{"temperature": 0.5},
	}
	wire := buildOpenAIRequest("gpt-4o-mini", req)
	if wire.Model != "gpt-4o-mini" || len(wire.Messages) != 1 || wire.Messages[0].Content != "hello" {
		t.Errorf("buildOpenAIRequest = %+v", wire)
	}
	if wire.Temperature != 0.5 {
		t.Errorf("temperature not propagated: %v", wire.Temperature)
	}

	body := `{"choices":[{"message":{"role":"assistant","content":"hi there"},"finish_reason":"stop"}],"usage":{"prompt_tokens":3,"completion_tokens":2}}`
	resp, err := parseOpenAIResponse([]byte(body))
	if err != nil || resp.Text() != "hi there" || resp.Finish != "stop" || resp.Usage.OutputTokens != 2 {
		t.Errorf("parseOpenAIResponse = %+v, %v", resp, err)
	}
	// Malformed / empty choices → CodeParse.
	if _, err := parseOpenAIResponse([]byte(`{bad`)); !asAPICode(err, CodeParse) {
		t.Error("malformed body should be CodeParse")
	}
	if _, err := parseOpenAIResponse([]byte(`{"choices":[]}`)); !asAPICode(err, CodeParse) {
		t.Error("empty choices should be CodeParse")
	}
}

func TestOpenAICompleteHTTP(t *testing.T) {
	t.Parallel()
	// Success server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"pong"},"finish_reason":"stop"}],"usage":{}}`))
	}))
	defer srv.Close()
	p := newOpenAI(ProviderConfig{Endpoint: srv.URL, Model: "m"})
	resp, err := p.Complete(context.Background(), CanonicalRequest{Messages: []CanonicalMessage{TextMessage(RoleUser, "ping")}})
	if err != nil || resp.Text() != "pong" {
		t.Errorf("Complete = %+v, %v", resp, err)
	}

	// 500 server → CodeHTTP5xx (INV-5).
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	p2 := newOpenAI(ProviderConfig{Endpoint: bad.URL, Model: "m"})
	if _, err := p2.Complete(context.Background(), CanonicalRequest{}); !asAPICode(err, CodeHTTP5xx) {
		t.Errorf("500 should map to CodeHTTP5xx, got %v", err)
	}
}

func TestFakeProvider(t *testing.T) {
	t.Parallel()
	f := NewFakeProvider("answer")
	resp, err := f.Complete(context.Background(), CanonicalRequest{})
	if err != nil || resp.Text() != "answer" {
		t.Errorf("Complete = %+v, %v", resp, err)
	}
	var got string
	if err := f.Stream(context.Background(), CanonicalRequest{}, func(c Chunk) error { got = c.Delta; return nil }); err != nil || got != "answer" {
		t.Errorf("Stream delta = %q, %v", got, err)
	}
	if f.Calls() != 2 {
		t.Errorf("Calls = %d, want 2", f.Calls())
	}
	// Error path.
	f.Err = errors.New("boom")
	if _, err := f.Complete(context.Background(), CanonicalRequest{}); err == nil {
		t.Error("Err should propagate")
	}
}

func TestResolveSecretAndRedact(t *testing.T) {
	// Not parallel: t.Setenv is incompatible with t.Parallel.
	t.Setenv("AIAPI_TEST_KEY", "sk-secret-123")
	secret, err := resolveSecret("env:AIAPI_TEST_KEY")
	if err != nil || string(secret) != "sk-secret-123" {
		t.Fatalf("resolveSecret = %q, %v", secret, err)
	}
	// INV-1: redactor masks the key.
	r := newRedactor(secret)
	if got := r.Redact("authorization: Bearer sk-secret-123"); got != "authorization: Bearer ***" {
		t.Errorf("redact = %q", got)
	}
	zero(secret)
	if secret[0] != 0 {
		t.Error("zero should wipe the secret buffer")
	}
	// Missing env → CodeMissingKey.
	if _, err := resolveSecret("env:NOPE_MISSING"); !asAPICode(err, CodeMissingKey) {
		t.Errorf("missing env err = %v", err)
	}
	// keyring/file → unsupported (ADR-gated).
	if _, err := resolveSecret("keyring:x"); !asAPICode(err, CodeMissingKey) {
		t.Errorf("keyring err = %v", err)
	}
}

func TestSelectAndRegistry(t *testing.T) {
	t.Parallel()
	// openai is registered via init.
	found := false
	for _, n := range RegisteredProviders() {
		if n == "openai" {
			found = true
		}
	}
	if !found {
		t.Error("openai should be registered")
	}
	cfg := DefaultConfig()
	cfg.ActiveProvider = "openai"
	cfg.Providers["openai"] = ProviderConfig{Endpoint: "http://x", Model: "m"}
	if _, err := Select(cfg); err != nil {
		t.Errorf("Select(openai) = %v", err)
	}
	cfg.ActiveProvider = "ghost"
	if _, err := Select(cfg); !errors.As(err, new(ErrUnsupportedProvider)) {
		t.Errorf("Select(ghost) = %v, want ErrUnsupportedProvider", err)
	}
}

// asAPICode reports whether err is an APIError with the given code.
func asAPICode(err error, code Code) bool {
	var e APIError
	return errors.As(err, &e) && e.Code == code
}
