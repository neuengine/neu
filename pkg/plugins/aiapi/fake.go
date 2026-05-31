//go:build editor

package aiapi

import "context"

// FakeProvider is an in-memory Provider for deterministic tests (L1 §4.9): it
// returns canned responses so method dispatch, streaming, and error handling can
// be tested without a live backend. It is exported so the assistant/plugin test
// harnesses can reuse it.
type FakeProvider struct {
	ProviderName string
	Reply        string // text returned by Complete/Stream
	Err          error  // if set, Complete/Stream return it
	calls        int
}

// NewFakeProvider returns a fake that replies with reply.
func NewFakeProvider(reply string) *FakeProvider {
	return &FakeProvider{ProviderName: "fake", Reply: reply}
}

// Name returns the provider name.
func (f *FakeProvider) Name() string {
	if f.ProviderName == "" {
		return "fake"
	}
	return f.ProviderName
}

// Calls returns how many times Complete/Stream were invoked.
func (f *FakeProvider) Calls() int { return f.calls }

// Complete returns the canned reply (or the configured error), honouring ctx.
func (f *FakeProvider) Complete(ctx context.Context, _ CanonicalRequest) (CanonicalResponse, error) {
	f.calls++
	if err := ctx.Err(); err != nil {
		return CanonicalResponse{}, APIError{Code: CodeCancelled, Message: err.Error()}
	}
	if f.Err != nil {
		return CanonicalResponse{}, f.Err
	}
	return CanonicalResponse{
		Message: TextMessage(RoleAssistant, f.Reply),
		Finish:  "stop",
		Usage:   Usage{InputTokens: 1, OutputTokens: len(f.Reply)},
	}, nil
}

// Stream emits the canned reply as a single final chunk.
func (f *FakeProvider) Stream(ctx context.Context, _ CanonicalRequest, sink func(Chunk) error) error {
	f.calls++
	if f.Err != nil {
		return f.Err
	}
	return sink(Chunk{Delta: f.Reply, Final: true})
}

// Embeddings is unsupported by the fake.
func (f *FakeProvider) Embeddings(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnsupported
}

var _ Provider = (*FakeProvider)(nil)
