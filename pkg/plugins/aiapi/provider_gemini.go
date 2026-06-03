//go:build editor

package aiapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func init() { RegisterProvider("gemini", newGemini) }

// geminiProvider speaks Google's generateContent API. It uses `contents` with
// `parts`, role "model" for the assistant, and a separate `systemInstruction`.
type geminiProvider struct {
	cfg    ProviderConfig
	client *http.Client
}

func newGemini(cfg ProviderConfig) Provider {
	return &geminiProvider{cfg: cfg, client: http.DefaultClient}
}

func (p *geminiProvider) Name() string { return "gemini" }

// --- wire types (Gemini generateContent) ---

type genPart struct {
	Text string `json:"text"`
}

type genContent struct {
	Role  string    `json:"role,omitempty"`
	Parts []genPart `json:"parts"`
}

type genRequest struct {
	SystemInstruction *genContent  `json:"systemInstruction,omitempty"`
	Contents          []genContent `json:"contents"`
}

type genResponse struct {
	Candidates []struct {
		Content      genContent `json:"content"`
		FinishReason string     `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	} `json:"usageMetadata"`
}

// geminiRole maps a canonical role to Gemini's vocabulary ("assistant" → "model").
func geminiRole(r Role) string {
	if r == RoleAssistant {
		return "model"
	}
	return "user" // user/tool collapse to "user" for the chat-completions subset
}

// buildGeminiRequest translates a canonical request to the Gemini wire shape
// (pure). System messages become `systemInstruction`; user/assistant turns
// become `contents` with mapped roles.
func buildGeminiRequest(r CanonicalRequest) genRequest {
	var req genRequest
	var system strings.Builder
	for _, m := range r.Messages {
		text := messageText(m)
		if m.Role == RoleSystem {
			system.WriteString(text)
			continue
		}
		req.Contents = append(req.Contents, genContent{
			Role:  geminiRole(m.Role),
			Parts: []genPart{{Text: text}},
		})
	}
	if s := system.String(); s != "" {
		req.SystemInstruction = &genContent{Parts: []genPart{{Text: s}}}
	}
	return req
}

// parseGeminiResponse translates a Gemini response to canonical (pure). Parts of
// the first candidate are concatenated; no candidates is a CodeParse error.
func parseGeminiResponse(body []byte) (CanonicalResponse, error) {
	var w genResponse
	if err := json.Unmarshal(body, &w); err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "decode response: " + err.Error()}
	}
	if len(w.Candidates) == 0 {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "response has no candidates"}
	}
	cand := w.Candidates[0]
	var text strings.Builder
	for _, part := range cand.Content.Parts {
		text.WriteString(part.Text)
	}
	return CanonicalResponse{
		Message: TextMessage(RoleAssistant, text.String()),
		Finish:  cand.FinishReason,
		Usage:   Usage{InputTokens: w.UsageMetadata.PromptTokenCount, OutputTokens: w.UsageMetadata.CandidatesTokenCount},
	}, nil
}

// Complete POSTs to the model's generateContent endpoint and parses the response.
func (p *geminiProvider) Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error) {
	wire := buildGeminiRequest(r)
	body, err := json.Marshal(wire)
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: err.Error()}
	}
	url := p.cfg.Endpoint + "/models/" + p.cfg.Model + ":generateContent"
	data, err := postJSON(ctx, p.client, url, body, p.cfg.ExtraHeaders)
	if err != nil {
		return CanonicalResponse{}, err
	}
	return parseGeminiResponse(data)
}

// Stream falls back to a single-chunk Complete (native Gemini SSE is a refinement).
func (p *geminiProvider) Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error {
	return completeAsStream(ctx, p, r, sink)
}

// Embeddings is unsupported by the chat provider.
func (p *geminiProvider) Embeddings(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

var _ Provider = (*geminiProvider)(nil)
