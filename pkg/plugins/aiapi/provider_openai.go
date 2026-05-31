//go:build editor

package aiapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func init() { RegisterProvider("openai", newOpenAI) }

// openAIProvider speaks the OpenAI-compatible chat-completions schema — the
// lingua franca that also covers local servers (ollama, llama.cpp, vLLM).
type openAIProvider struct {
	cfg    ProviderConfig
	client *http.Client
}

func newOpenAI(cfg ProviderConfig) Provider {
	return &openAIProvider{cfg: cfg, client: http.DefaultClient}
}

func (p *openAIProvider) Name() string { return "openai" }

// --- wire types (OpenAI-compatible) ---

type oaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type oaiRequest struct {
	Model       string         `json:"model"`
	Messages    []oaiMessage   `json:"messages"`
	Temperature any            `json:"temperature,omitempty"`
	Extra       map[string]any `json:"-"`
}

type oaiResponse struct {
	Choices []struct {
		Message      oaiMessage `json:"message"`
		FinishReason string     `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

// buildOpenAIRequest translates a canonical request to the OpenAI wire shape
// (pure; testable without HTTP). Text content parts are concatenated.
func buildOpenAIRequest(model string, r CanonicalRequest) oaiRequest {
	msgs := make([]oaiMessage, len(r.Messages))
	for i, m := range r.Messages {
		var content strings.Builder
		for _, part := range m.Content {
			if part.Kind == "text" {
				content.WriteString(part.Text)
			}
		}
		msgs[i] = oaiMessage{Role: string(m.Role), Content: content.String()}
	}
	req := oaiRequest{Model: model, Messages: msgs}
	if t, ok := r.Parameters["temperature"]; ok {
		req.Temperature = t
	}
	return req
}

// parseOpenAIResponse translates an OpenAI wire response to canonical (pure).
// A malformed body or empty choices is a CodeParse error (INV-5).
func parseOpenAIResponse(body []byte) (CanonicalResponse, error) {
	var w oaiResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "decode response: " + err.Error()}
	}
	if len(w.Choices) == 0 {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "response has no choices"}
	}
	c := w.Choices[0]
	return CanonicalResponse{
		Message: TextMessage(RoleAssistant, c.Message.Content),
		Finish:  c.FinishReason,
		Usage:   Usage{InputTokens: w.Usage.PromptTokens, OutputTokens: w.Usage.CompletionTokens},
	}, nil
}

// Complete builds the request, POSTs it, maps HTTP errors (INV-5), and parses
// the canonical response.
func (p *openAIProvider) Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error) {
	wire := buildOpenAIRequest(p.cfg.Model, r)
	body, err := json.Marshal(wire)
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: err.Error()}
	}
	url := p.cfg.Endpoint + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeUnknown, Message: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	for k, v := range p.cfg.ExtraHeaders {
		httpReq.Header.Set(k, v)
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return CanonicalResponse{}, APIError{Code: CodeTimeout, Message: ctx.Err().Error()}
		}
		return CanonicalResponse{}, APIError{Code: CodeHTTP5xx, Message: err.Error()}
	}
	defer resp.Body.Close()
	if code, isErr := MapHTTPStatus(resp.StatusCode); isErr {
		return CanonicalResponse{}, APIError{Code: code, Message: fmt.Sprintf("provider HTTP %d", resp.StatusCode)}
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: err.Error()}
	}
	return parseOpenAIResponse(respBody)
}

// Stream is deferred for Bootstrap (chat streaming lands with SSE wiring).
func (p *openAIProvider) Stream(context.Context, CanonicalRequest, func(Chunk) error) error {
	return ErrUnsupported
}

// Embeddings is not implemented by the chat provider.
func (p *openAIProvider) Embeddings(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

var _ Provider = (*openAIProvider)(nil)
