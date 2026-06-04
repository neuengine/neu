//go:build editor && live_ai

// Package-level live smoke test for the AI API plugin (T-6T05). It is gated by
// BOTH the `editor` and `live_ai` build tags, so neither the default `go test
// ./...` nor an `-tags editor` run ever compiles it — only the ai-live.yml CI
// workflow (PR label `live-ai`, project-secret API key) does. The request is
// cost-bounded: a one-word prompt, a cheap model, and a capped output.
package aiapi

import (
	"context"
	"os"
	"testing"
	"time"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// TestLiveSmoke hits a real OpenAI-compatible provider with a minimal request
// and asserts a non-empty canonical response. It skips (does not fail) when the
// API key is absent, so an accidental tagged run without secrets is harmless.
func TestLiveSmoke(t *testing.T) {
	key := os.Getenv("AIAPI_LIVE_KEY")
	if key == "" {
		t.Skip("AIAPI_LIVE_KEY not set — live smoke skipped (only the live-ai CI job sets it)")
	}

	cfg := DefaultConfig()
	cfg.ActiveProvider = "openai"
	cfg.Providers["openai"] = ProviderConfig{
		Endpoint: envOr("AIAPI_LIVE_ENDPOINT", "https://api.openai.com/v1"),
		Model:    envOr("AIAPI_LIVE_MODEL", "gpt-4o-mini"),
		// The OpenAI provider authenticates via ExtraHeaders; inject the bearer
		// token directly rather than going through the plugin credential lifecycle.
		ExtraHeaders: map[string]string{"Authorization": "Bearer " + key},
	}

	provider, err := Select(cfg)
	if err != nil {
		t.Fatalf("Select(openai): %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := provider.Complete(ctx, CanonicalRequest{
		Messages: []CanonicalMessage{
			TextMessage(RoleUser, "Reply with the single word: pong"),
		},
		// Cost budget: cap the output to a handful of tokens.
		Parameters: map[string]any{"max_output_tokens": 5},
	})
	if err != nil {
		t.Fatalf("live Complete: %v", err)
	}
	if messageText(resp.Message) == "" {
		t.Error("live provider returned an empty response")
	}
	t.Logf("live smoke ok: reply=%q usage(in=%d out=%d)",
		messageText(resp.Message), resp.Usage.InputTokens, resp.Usage.OutputTokens)
}
