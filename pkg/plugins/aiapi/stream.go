//go:build editor

package aiapi

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// maxSSELine bounds a single SSE data line. Model deltas are small, but a full
// final chunk with tool-call payloads can be large; 1 MiB is generous and caps
// a hostile/runaway stream (defence-in-depth alongside the request timeout).
const maxSSELine = 1 << 20

// oaiStreamChunk is one OpenAI-compatible streaming delta (the `data:` payload
// of a chat-completions SSE event). FinishReason is a pointer so a null (mid-
// stream) is distinguishable from "stop" (final).
type oaiStreamChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

// decodeSSE reads an OpenAI-style Server-Sent-Events stream from r and calls
// sink for each content delta (L1 §4.6). It is pure with respect to transport —
// the caller supplies the response body — so it is unit-testable without HTTP.
//
// Termination: the sentinel "data: [DONE]" line, a chunk whose finish_reason is
// set (delivered as the final Chunk), a sink error, or ctx cancellation. ctx is
// checked between events so a consumer can cancel at any chunk boundary (INV-6);
// a cancelled context returns CodeCancelled, a malformed chunk returns CodeParse
// (INV-5).
func decodeSSE(ctx context.Context, r io.Reader, sink func(Chunk) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), maxSSELine)

	for sc.Scan() {
		if err := ctx.Err(); err != nil {
			return APIError{Code: CodeCancelled, Message: err.Error()}
		}
		line := strings.TrimSpace(sc.Text())
		// Blank separators, SSE comments (":..."), and non-data fields
		// ("event:", "id:") are not content — skip them.
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			return nil
		}

		var chunk oaiStreamChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return APIError{Code: CodeParse, Message: "decode stream chunk: " + err.Error()}
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		ch := chunk.Choices[0]
		final := ch.FinishReason != nil && *ch.FinishReason != ""
		// Role-only / keepalive deltas carry no text and aren't final — skip.
		if ch.Delta.Content == "" && !final {
			continue
		}
		if err := sink(Chunk{Delta: ch.Delta.Content, Final: final}); err != nil {
			return err
		}
		if final {
			return nil
		}
	}

	if err := sc.Err(); err != nil {
		if ctx.Err() != nil {
			return APIError{Code: CodeCancelled, Message: ctx.Err().Error()}
		}
		return APIError{Code: CodeParse, Message: "scan stream: " + err.Error()}
	}
	return nil
}
