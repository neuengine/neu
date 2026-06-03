//go:build editor

package aiapi

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

// postJSON POSTs body to url with Content-Type application/json plus headers,
// maps the HTTP status to a coded APIError (INV-5), and returns the response
// body. A cancelled/timed-out context maps to CodeCancelled; a transport error
// to CodeHTTP5xx; a non-2xx status to its mapped code. Shared by every provider
// so the request/error-mapping flow lives in one place.
func postJSON(ctx context.Context, client *http.Client, url string, body []byte, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, APIError{Code: CodeUnknown, Message: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, APIError{Code: CodeCancelled, Message: ctx.Err().Error()}
		}
		return nil, APIError{Code: CodeHTTP5xx, Message: err.Error()}
	}
	defer resp.Body.Close()
	if code, isErr := MapHTTPStatus(resp.StatusCode); isErr {
		return nil, APIError{Code: code, Message: fmt.Sprintf("provider HTTP %d", resp.StatusCode)}
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, APIError{Code: CodeParse, Message: "read response: " + err.Error()}
	}
	return data, nil
}

// completeAsStream adapts a non-streaming provider to the Stream contract: it
// runs one Complete and delivers the whole result as a single final chunk. Used
// by providers whose native streaming format is not yet decoded (Anthropic,
// Gemini) so chat still works through any provider (just not incrementally).
func completeAsStream(ctx context.Context, p Provider, r CanonicalRequest, sink func(Chunk) error) error {
	resp, err := p.Complete(ctx, r)
	if err != nil {
		return err
	}
	return sink(Chunk{Delta: resp.Text(), Final: true})
}
