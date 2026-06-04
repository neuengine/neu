//go:build editor

package aiapi

import (
	"context"
	"fmt"
	"testing"
)

// T-6T02 AI-API parity matrix: every standard assistant method is exercised in
// both modes — in-process and across the out-of-process canonical JSON boundary —
// and the canonical response must be identical (INV-7). The OOP loader drives
// lifecycle only (no method RPC yet), so "out-of-process" here is the canonical
// wire crossing (completeOverWire / a wire-wrapping AIService); full
// subprocess-RPC parity lands when OOP method dispatch is built.

// echoProvider reflects the request's user message into the response and sets
// deterministic usage, so the canonical round-trip is genuinely exercised — a
// fixed-reply fake would pass parity trivially regardless of request shape.
type echoProvider struct{ name string }

func (e echoProvider) Name() string { return e.name }

func (e echoProvider) Complete(_ context.Context, r CanonicalRequest) (CanonicalResponse, error) {
	var user string
	for _, m := range r.Messages {
		if m.Role == RoleUser {
			user = messageText(m)
		}
	}
	return CanonicalResponse{
		Message: TextMessage(RoleAssistant, "echo:"+user),
		Finish:  "stop",
		Usage:   Usage{InputTokens: len(user), OutputTokens: len(user) + 5},
	}, nil
}

func (e echoProvider) Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error {
	resp, err := e.Complete(ctx, r)
	if err != nil {
		return err
	}
	return sink(Chunk{Delta: messageText(resp.Message), Final: true})
}

func (e echoProvider) Embeddings(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

var _ Provider = echoProvider{}

// wireService is an AIService that routes every call through the OOP canonical
// wire boundary (completeOverWire), so a Dispatcher built over it runs each
// method "out-of-process". Mirrors the readyService (in-process) test helper.
type wireService struct{ p Provider }

func (w wireService) Ready() bool { return true }

func (w wireService) Complete(ctx context.Context, r CanonicalRequest) (CanonicalResponse, error) {
	return completeOverWire(ctx, w.p, r)
}

func (w wireService) Stream(ctx context.Context, r CanonicalRequest, sink func(Chunk) error) error {
	resp, err := completeOverWire(ctx, w.p, r)
	if err != nil {
		return err
	}
	return sink(Chunk{Delta: messageText(resp.Message), Final: true})
}

var _ AIService = wireService{}

// TestParityMatrix_CanonicalBoundary runs CheckParity for the canonical request
// shape of every standard method — proving each method's request + response is
// mode-invariant across the JSON wire.
func TestParityMatrix_CanonicalBoundary(t *testing.T) {
	t.Parallel()
	p := echoProvider{name: "echo"}
	for method, spec := range methodSpecs {
		req := CanonicalRequest{
			Messages: []CanonicalMessage{
				TextMessage(RoleSystem, spec.system),
				TextMessage(RoleUser, "representative input for "+method),
			},
		}
		if err := CheckParity(t.Context(), p, req); err != nil {
			t.Errorf("method %q: canonical-boundary parity failed: %v", method, err)
		}
	}
	if len(methodSpecs) != 10 {
		t.Errorf("methodSpecs has %d methods, want 10 (parity matrix may be stale)", len(methodSpecs))
	}
}

// TestParityMatrix_DispatchBothModes dispatches every standard method through a
// Dispatcher built over an in-process service and over a wire-wrapping service,
// asserting the canonical response is identical — the full routing path
// (extractInput, system-prompt selection, chat→Stream) exercised in both modes.
func TestParityMatrix_DispatchBothModes(t *testing.T) {
	t.Parallel()
	p := echoProvider{name: "echo"}
	direct := NewDispatcher(readyService(p), nil, nil, nil)
	wired := NewDispatcher(wireService{p: p}, nil, nil, nil)

	for method := range methodSpecs {
		params := map[string]any{"prompt": "do the thing for " + method}
		inResp, inErr := direct.Dispatch(t.Context(), "req-1", method, params)
		wireResp, wireErr := wired.Dispatch(t.Context(), "req-1", method, params)
		if (inErr == nil) != (wireErr == nil) {
			t.Errorf("method %q: error mismatch in=%v wire=%v", method, inErr, wireErr)
			continue
		}
		if inErr != nil {
			continue
		}
		if messageText(inResp.Message) != messageText(wireResp.Message) || inResp.Finish != wireResp.Finish {
			t.Errorf("method %q: dispatch response differs across modes:\n in=%+v\nwire=%+v",
				method, inResp, wireResp)
		}
	}
}

// flakyProvider returns a different reply on each call, so the in-process and
// over-wire invocations diverge — proving CheckParity actually detects a
// non-mode-invariant provider rather than passing vacuously.
type flakyProvider struct{ n int }

func (f *flakyProvider) Name() string { return "flaky" }

func (f *flakyProvider) Complete(context.Context, CanonicalRequest) (CanonicalResponse, error) {
	f.n++
	return CanonicalResponse{Message: TextMessage(RoleAssistant, fmt.Sprintf("reply-%d", f.n))}, nil
}

func (f *flakyProvider) Stream(context.Context, CanonicalRequest, func(Chunk) error) error {
	return ErrUnsupported
}

func (f *flakyProvider) Embeddings(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

func TestParityDetectsDivergence(t *testing.T) {
	t.Parallel()
	err := CheckParity(t.Context(), &flakyProvider{}, CanonicalRequest{
		Messages: []CanonicalMessage{TextMessage(RoleUser, "hi")},
	})
	if err == nil {
		t.Error("CheckParity must detect a non-mode-invariant (flaky) provider")
	}
}
