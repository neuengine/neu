//go:build editor

package aiapi

import (
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"strings"
)

func init() { RegisterProvider("anthropic", newAnthropic) }

// anthropicVersion is the required Messages API version header.
const anthropicVersion = "2023-06-01"

// defaultMaxTokens is Anthropic's required max_tokens when the caller gives none.
const defaultMaxTokens = 1024

// anthropicProvider speaks the Anthropic Messages API. Unlike OpenAI it hoists
// the system prompt to a top-level field and requires max_tokens.
type anthropicProvider struct {
	cfg    ProviderConfig
	client *http.Client
}

func newAnthropic(cfg ProviderConfig) Provider {
	return &anthropicProvider{cfg: cfg, client: http.DefaultClient}
}

func (p *anthropicProvider) Name() string { return "anthropic" }

// --- wire types (Anthropic Messages API) ---

type antMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type antRequest struct {
	Model     string       `json:"model"`
	System    string       `json:"system,omitempty"`
	Messages  []antMessage `json:"messages"`
	MaxTokens int          `json:"max_tokens"`
}

type antResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// buildAnthropicRequest translates a canonical request to the Anthropic wire
// shape (pure). System messages are hoisted to the top-level `system` field;
// only user/assistant turns go in `messages`. max_tokens comes from the
// canonical `max_output_tokens` parameter, else the default.
func buildAnthropicRequest(model string, r CanonicalRequest) antRequest {
	req := antRequest{Model: model, MaxTokens: defaultMaxTokens}
	if mt, ok := intParam(r.Parameters, "max_output_tokens"); ok {
		req.MaxTokens = mt
	}
	var system strings.Builder
	for _, m := range r.Messages {
		text := messageText(m)
		if m.Role == RoleSystem {
			system.WriteString(text)
			continue
		}
		req.Messages = append(req.Messages, antMessage{Role: string(m.Role), Content: text})
	}
	req.System = system.String()
	return req
}

// parseAnthropicResponse translates an Anthropic response to canonical (pure).
// Text blocks are concatenated; a missing/empty content is a CodeParse error.
func parseAnthropicResponse(body []byte) (CanonicalResponse, error) {
	var w antResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "decode response: " + err.Error()}
	}
	if len(w.Content) == 0 {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "response has no content blocks"}
	}
	var text strings.Builder
	for _, c := range w.Content {
		if c.Type == "text" {
			text.WriteString(c.Text)
		}
	}
	return CanonicalResponse{
		Message: TextMessage(RoleAssistant, text.String()),
		Finish:  w.StopReason,
		Usage:   Usage{InputTokens: w.Usage.InputTokens, OutputTokens: w.Usage.OutputTokens},
	}, nil
}

// Complete POSTs the Messages request and parses the canonical response. The
// anthropic-version header is always set; auth flows through ExtraHeaders.
func (p *anthropicProvider) Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error) {
	wire := buildAnthropicRequest(p.cfg.Model, r)
	body, err := json.Marshal(wire)
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: err.Error()}
	}
	headers := map[string]string{"anthropic-version": anthropicVersion}
	maps.Copy(headers, p.cfg.ExtraHeaders)
	data, err := postJSON(ctx, p.client, p.cfg.Endpoint+"/messages", body, headers)
	if err != nil {
		return CanonicalResponse{}, err
	}
	return parseAnthropicResponse(data)
}

// Stream falls back to a single-chunk Complete (native Anthropic SSE decoding is
// a refinement) so chat works through this provider.
func (p *anthropicProvider) Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error {
	return completeAsStream(ctx, p, r, sink)
}

// Embeddings is unsupported by the messages provider.
func (p *anthropicProvider) Embeddings(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

var _ Provider = (*anthropicProvider)(nil)
