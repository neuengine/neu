//go:build editor

// Package aiapi is the first-party AI API plugin: a unified client over
// OpenAI-compatible chat-completions providers (OpenAI, Anthropic, Gemini,
// local). Provider wire formats are translated to/from the internal canonical
// types so only canonical types cross plugin boundaries. Editor-only
// (//go:build editor) per l1-ai-assistant-system §2.
//
// Bootstrap: l2-ai-api-plugin-go Draft (Phase 6 Track O, C29 gate open).
package aiapi

import "strings"

// Role is a canonical message role.
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// ContentPart is one piece of a message's content (text, image, tool result).
type ContentPart struct {
	Kind string `json:"kind"` // "text" | "image_url" | "tool_use" | "tool_result"
	Text string `json:"text,omitempty"`
	URL  string `json:"url,omitempty"`
}

// CanonicalMessage is a provider-agnostic chat message.
type CanonicalMessage struct {
	Role    Role          `json:"role"`
	Content []ContentPart `json:"content"`
}

// TextMessage builds a single-text-part message.
func TextMessage(role Role, text string) CanonicalMessage {
	return CanonicalMessage{Role: role, Content: []ContentPart{{Kind: "text", Text: text}}}
}

// CanonicalRequest is the provider-agnostic completion request.
type CanonicalRequest struct {
	Messages   []CanonicalMessage `json:"messages"`
	Parameters map[string]any     `json:"parameters,omitempty"` // temperature, top_p, stop, ...
}

// Usage reports token consumption + optional cost for diagnostics (INV-9).
type Usage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
}

// CanonicalResponse is the provider-agnostic completion result.
type CanonicalResponse struct {
	Message CanonicalMessage `json:"message"`
	Finish  string           `json:"finish"` // stop | length | tool_use | filter
	Usage   Usage            `json:"usage"`
}

// Text returns the concatenated text content of the response message.
func (r CanonicalResponse) Text() string {
	var s strings.Builder
	for _, p := range r.Message.Content {
		if p.Kind == "text" {
			s.WriteString(p.Text)
		}
	}
	return s.String()
}
