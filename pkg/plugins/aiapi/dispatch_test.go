//go:build editor

package aiapi

import (
	"context"
	"errors"
	"testing"

	"github.com/neuengine/neu/pkg/assistant"
)

// recordingRecorder captures RecordUsage calls.
type recordingRecorder struct {
	usages []Usage
}

func (r *recordingRecorder) RecordUsage(_ string, u Usage) { r.usages = append(r.usages, u) }

// readyService returns a ServiceRegistry activated with the given provider.
func readyService(p Provider) *ServiceRegistry {
	s := NewServiceRegistry()
	s.activate(p)
	return s
}

func TestDispatch_UnknownMethod(t *testing.T) {
	t.Parallel()
	d := NewDispatcher(readyService(NewFakeProvider("x")), nil, nil, nil)
	_, err := d.Dispatch(context.Background(), "req-1", "com.example.custom", nil)
	if !asAPICode(err, CodeCapabilityDeny) {
		t.Errorf("unknown method = %v, want CodeCapabilityDeny", err)
	}
}

func TestDispatch_ServiceNotReady(t *testing.T) {
	t.Parallel()
	d := NewDispatcher(NewServiceRegistry(), nil, nil, nil) // not activated
	_, err := d.Dispatch(context.Background(), "req-1", assistant.MethodChat, nil)
	if !errors.Is(err, ErrServiceNotReady) {
		t.Errorf("not-ready dispatch = %v, want ErrServiceNotReady", err)
	}
}

func TestDispatch_CompleteMethodRecordsUsage(t *testing.T) {
	t.Parallel()
	fake := NewFakeProvider("a list of components")
	rec := &recordingRecorder{}
	d := NewDispatcher(readyService(fake), nil, rec, nil)

	resp, err := d.Dispatch(context.Background(), "req-1", assistant.MethodSuggestComponents,
		map[string]any{"description": "a player"})
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if resp.Text() != "a list of components" {
		t.Errorf("response text = %q", resp.Text())
	}
	if len(rec.usages) != 1 || rec.usages[0].OutputTokens == 0 {
		t.Errorf("usage not recorded (INV-9): %+v", rec.usages)
	}
	// suggest_components is non-streaming → Complete called once.
	if fake.Calls() != 1 {
		t.Errorf("provider calls = %d, want 1 (Complete)", fake.Calls())
	}
}

func TestDispatch_ChatStreamsToEmitter(t *testing.T) {
	t.Parallel()
	fake := NewFakeProvider("streamed answer")
	var events []StreamEvent
	emit := func(e StreamEvent) { events = append(events, e) }
	d := NewDispatcher(readyService(fake), nil, &recordingRecorder{}, emit)

	resp, err := d.Dispatch(context.Background(), "req-42", assistant.MethodChat,
		map[string]any{"prompt": "hi"})
	if err != nil {
		t.Fatalf("Dispatch chat: %v", err)
	}
	if resp.Text() != "streamed answer" {
		t.Errorf("accumulated chat text = %q", resp.Text())
	}
	if len(events) != 1 || events[0].RequestID != "req-42" || !events[0].Final {
		t.Errorf("stream events = %+v, want one final event tagged req-42 (INV-6)", events)
	}
}

func TestDispatch_RateLimited(t *testing.T) {
	t.Parallel()
	rl := newRateLimiter(1, 0) // 1 request/min
	d := NewDispatcher(readyService(NewFakeProvider("x")), rl, nil, nil)

	// First passes, second is throttled (INV-8).
	if _, err := d.Dispatch(context.Background(), "r1", assistant.MethodChat, map[string]any{"prompt": "a"}); err != nil {
		t.Fatalf("first dispatch: %v", err)
	}
	_, err := d.Dispatch(context.Background(), "r2", assistant.MethodChat, map[string]any{"prompt": "b"})
	if !asAPICode(err, CodeRateLimited) {
		t.Errorf("second dispatch = %v, want CodeRateLimited (INV-8)", err)
	}
}

func TestDispatch_ProviderError(t *testing.T) {
	t.Parallel()
	fake := NewFakeProvider("x")
	fake.Err = APIError{Code: CodeHTTP5xx, Message: "boom"}
	d := NewDispatcher(readyService(fake), nil, nil, nil)
	if _, err := d.Dispatch(context.Background(), "r1", assistant.MethodSuggestComponents, nil); !asAPICode(err, CodeHTTP5xx) {
		t.Errorf("provider error = %v, want CodeHTTP5xx", err)
	}
}

func TestExtractInput(t *testing.T) {
	t.Parallel()
	// Declared key wins.
	if got := extractInput("description", map[string]any{"description": "d", "prompt": "p"}); got != "d" {
		t.Errorf("declared key = %q, want d", got)
	}
	// Fallback to "prompt".
	if got := extractInput("description", map[string]any{"prompt": "p"}); got != "p" {
		t.Errorf("prompt fallback = %q, want p", got)
	}
	// Join of string params when no known key present.
	got := extractInput("missing", map[string]any{"a": "x", "n": 5})
	if got != "x" {
		t.Errorf("join fallback = %q, want x (non-string skipped)", got)
	}
	// Nil params → empty.
	if got := extractInput("k", nil); got != "" {
		t.Errorf("nil params = %q, want empty", got)
	}
}
