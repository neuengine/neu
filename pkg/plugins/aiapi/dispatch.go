//go:build editor

package aiapi

import (
	"context"
	"strings"

	"github.com/neuengine/neu/pkg/assistant"
)

// StreamEvent is one chat streaming delta the dispatcher forwards to the editor
// (L1 §4.6). The app wires the emitter to the ECS event bus (an AssistantStream
// event readable next frame via the T-6X01 swap); the plugin stays decoupled
// from the bus by emitting through an injected callback.
type StreamEvent struct {
	RequestID string
	Delta     string
	Seq       int
	Final     bool
}

// StreamEmitter receives chat stream events. The app supplies a bus-backed
// emitter; a nil emitter disables emission (the deltas still accumulate into the
// returned response).
type StreamEmitter func(StreamEvent)

// methodSpec describes how a standard assistant method maps onto a provider call
// (L1 §4.5): the system prompt that frames the model, the params key carrying
// the user input, and whether the method streams (chat only).
type methodSpec struct {
	system  string
	userKey string
	stream  bool
}

// methodSpecs maps each standard method to its provider mapping. Methods absent
// here are rejected by Dispatch (a custom method needs explicit registration).
var methodSpecs = map[string]methodSpec{
	assistant.MethodChat:              {system: "You are a helpful game-development assistant for the Neu engine.", userKey: "prompt", stream: true},
	assistant.MethodSuggestComponents: {system: "Suggest ECS components for the described entity. Reply with a concise list.", userKey: "description"},
	assistant.MethodGenerateScene:     {system: "Generate a scene definition from the description. Reply with a definition document.", userKey: "description"},
	assistant.MethodGenerateUI:        {system: "Generate a UI definition from the description. Reply with a definition document.", userKey: "description"},
	assistant.MethodExplainEntity:     {system: "Explain what the described entity does based on its components.", userKey: "entity"},
	assistant.MethodDiagnoseIssue:     {system: "Analyze the diagnostic and suggest concrete fixes.", userKey: "diagnostic"},
	assistant.MethodOptimizeScene:     {system: "Suggest performance optimizations for the described scene.", userKey: "scene"},
	assistant.MethodGenerateCode:      {system: "Generate Go system code from the description. Reply with code only.", userKey: "description"},
	assistant.MethodReviewDefinition:  {system: "Review the definition for issues and reply with findings.", userKey: "definition"},
	assistant.MethodAutocomplete:      {system: "Complete the property value. Reply with the completion only.", userKey: "prefix"},
}

// Dispatcher routes standard assistant methods to the active AI provider through
// the ServiceRegistry, enforces the per-provider rate limit (INV-8), records
// token/cost usage (INV-9), and (for chat) streams deltas to an emitter (INV-6).
type Dispatcher struct {
	service  AIService
	limiter  *rateLimiter
	recorder UsageRecorder
	emit     StreamEmitter
}

// NewDispatcher builds a dispatcher over the AI service. limiter, recorder, and
// emit are optional (nil disables rate limiting / usage recording / streaming
// emission respectively) so the dispatcher is usable in minimal wirings + tests.
func NewDispatcher(service AIService, limiter *rateLimiter, recorder UsageRecorder, emit StreamEmitter) *Dispatcher {
	return &Dispatcher{service: service, limiter: limiter, recorder: recorder, emit: emit}
}

// Dispatch routes method+params to the provider and returns the canonical
// response. Unknown methods are rejected (CodeCapabilityDeny). chat streams its
// deltas to the emitter and accumulates them into the response text; all other
// methods complete in one shot. Usage is recorded on success (INV-9).
func (d *Dispatcher) Dispatch(ctx context.Context, requestID, method string, params map[string]any) (CanonicalResponse, error) {
	spec, ok := methodSpecs[method]
	if !ok {
		return CanonicalResponse{}, APIError{Code: CodeCapabilityDeny, Message: "unsupported method: " + method}
	}
	if !d.service.Ready() {
		return CanonicalResponse{}, ErrServiceNotReady
	}

	userInput := extractInput(spec.userKey, params)
	req := CanonicalRequest{
		Messages: []CanonicalMessage{
			TextMessage(RoleSystem, spec.system),
			TextMessage(RoleUser, userInput),
		},
	}

	// INV-8: client-side rate limit. Estimate tokens from input length (~4 chars
	// per token is the common heuristic) plus a small system-prompt allowance.
	if d.limiter != nil {
		estimate := (len(spec.system)+len(userInput))/4 + 1
		if okAllow, retry := d.limiter.Allow(estimate); !okAllow {
			return CanonicalResponse{}, APIError{Code: CodeRateLimited, Message: "rate limit exceeded", RetryAfter: retry}
		}
	}

	if spec.stream {
		return d.dispatchStream(ctx, requestID, req)
	}
	resp, err := d.service.Complete(ctx, req)
	if err != nil {
		return CanonicalResponse{}, err
	}
	d.record(resp.Usage)
	return resp, nil
}

// dispatchStream streams a chat response, forwarding each delta to the emitter
// with a monotonic sequence number and accumulating the full text into the
// returned response.
func (d *Dispatcher) dispatchStream(ctx context.Context, requestID string, req CanonicalRequest) (CanonicalResponse, error) {
	var (
		text strings.Builder
		seq  int
	)
	err := d.service.Stream(ctx, req, func(c Chunk) error {
		text.WriteString(c.Delta)
		if d.emit != nil {
			d.emit(StreamEvent{RequestID: requestID, Seq: seq, Delta: c.Delta, Final: c.Final})
		}
		seq++
		return nil
	})
	if err != nil {
		return CanonicalResponse{}, err
	}
	resp := CanonicalResponse{
		Message: TextMessage(RoleAssistant, text.String()),
		Finish:  "stop",
	}
	d.record(resp.Usage) // streaming usage is provider-dependent; zero by default
	return resp, nil
}

// record forwards usage to the recorder when one is wired (INV-9).
func (d *Dispatcher) record(u Usage) {
	if d.recorder != nil {
		d.recorder.RecordUsage("", u)
	}
}

// extractInput pulls the user input from params: the method's declared key
// first, then common fallbacks ("prompt"/"input"/"text"), then a join of all
// string params — so dispatch tolerates caller param-naming variation.
func extractInput(key string, params map[string]any) string {
	if params == nil {
		return ""
	}
	for _, k := range []string{key, "prompt", "input", "text"} {
		if v, ok := params[k].(string); ok && v != "" {
			return v
		}
	}
	var parts []string
	for _, v := range params {
		if s, ok := v.(string); ok && s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}
