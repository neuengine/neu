//go:build editor

// Package assistant implements the editor-facing AI agent integration: an
// AssistantManager that connects agents over pluggable transports (stdio /
// websocket / http), a capability model gating what each agent may do, and a
// modification path that routes all world edits through the standard Command
// pipeline tagged for undo. The entire package is editor-only (//go:build
// editor) — shipped games and headless builds link none of it.
//
// Bootstrap: l2-ai-assistant-system-go Draft (Phase 6 Track H, C29 gate open).
package assistant

import "encoding/json"

// AgentID identifies a connected agent; RequestID correlates a request with its
// response and groups the resulting world edits into one undo step (INV-4).
type (
	AgentID   string
	RequestID string
)

// MessageType discriminates the JSON protocol envelope (L1 §4.2).
type MessageType uint8

const (
	MsgRequest MessageType = iota
	MsgResponse
	MsgNotification
	MsgError
)

// AgentError is the error payload of an MsgError message.
type AgentError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *AgentError) Error() string { return e.Message }

// AgentMessage is the JSON envelope exchanged with an agent (L1 §4.2). It is
// MCP-shaped (id/type/method/params/result) so an MCP adapter is additive.
type AgentMessage struct {
	ID     string         `json:"id"`
	Type   MessageType    `json:"type"`
	Method string         `json:"method,omitempty"`
	Params map[string]any `json:"params,omitempty"`
	Result any            `json:"result,omitempty"`
	Error  *AgentError    `json:"error,omitempty"`
}

// Encode marshals a message to its newline-delimited JSON wire form.
func Encode(m AgentMessage) ([]byte, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Decode unmarshals one JSON message (the framing newline is stripped by the
// transport's line reader before this is called).
func Decode(line []byte) (AgentMessage, error) {
	var m AgentMessage
	err := json.Unmarshal(line, &m)
	return m, err
}
