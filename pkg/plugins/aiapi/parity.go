//go:build editor

package aiapi

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
)

// CheckParity verifies that a provider's canonical result is identical whether
// obtained in-process or across the out-of-process wire boundary (INV-7). The
// OOP path serializes the request to JSON (as it crosses to the subprocess),
// runs the provider, and serializes the response back (as it returns) — the same
// pkg/protocol JSON boundary an OOP plugin uses. A divergence means the canonical
// types are not mode-invariant, which is a bug.
//
// Use a deterministic provider (FakeProvider) so the two invocations are
// comparable. Full subprocess-RPC parity (driving the provider over a spawned
// pkg/protocol connection) lands when OOP method dispatch is built; this harness
// proves the canonical-boundary half today.
func CheckParity(ctx context.Context, p Provider, r CanonicalRequest) error {
	direct, err := p.Complete(ctx, r)
	if err != nil {
		return fmt.Errorf("aiapi parity: in-process call: %w", err)
	}
	wired, err := completeOverWire(ctx, p, r)
	if err != nil {
		return fmt.Errorf("aiapi parity: out-of-process call: %w", err)
	}
	if !reflect.DeepEqual(direct, wired) {
		return fmt.Errorf("aiapi parity: in-process %+v != out-of-process %+v (INV-7)", direct, wired)
	}
	return nil
}

// RunParity runs CheckParity for every request, returning the first divergence.
func RunParity(ctx context.Context, p Provider, reqs []CanonicalRequest) error {
	for i, r := range reqs {
		if err := CheckParity(ctx, p, r); err != nil {
			return fmt.Errorf("request %d: %w", i, err)
		}
	}
	return nil
}

// completeOverWire simulates the OOP round-trip: marshal the request to the
// protocol wire, unmarshal it (subprocess side), run the provider, then marshal
// and unmarshal the response (return side). Any JSON round-trip discrepancy in
// the canonical types surfaces here.
func completeOverWire(ctx context.Context, p Provider, r CanonicalRequest) (CanonicalResponse, error) {
	reqWire, err := json.Marshal(r)
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "marshal request: " + err.Error()}
	}
	var subReq CanonicalRequest
	if err := json.Unmarshal(reqWire, &subReq); err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "unmarshal request: " + err.Error()}
	}

	resp, err := p.Complete(ctx, subReq)
	if err != nil {
		return CanonicalResponse{}, err
	}

	respWire, err := json.Marshal(resp)
	if err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "marshal response: " + err.Error()}
	}
	var ret CanonicalResponse
	if err := json.Unmarshal(respWire, &ret); err != nil {
		return CanonicalResponse{}, APIError{Code: CodeParse, Message: "unmarshal response: " + err.Error()}
	}
	return ret, nil
}
