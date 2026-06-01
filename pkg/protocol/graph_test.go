package protocol_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"testing"

	"github.com/neuengine/neu/pkg/protocol"
)

func graphCases() []roundTripCase {
	frame := protocol.GraphExecutionFrame{
		PinValues: map[string]any{"a": float64(2), "msg": "hi"},
		NodeID:    "n1", NodeType: "math.Add", Timestamp: 1_000, StepIndex: 7,
	}
	return []roundTripCase{
		{protocol.KindGraphBreakpointHit, protocol.GraphBreakpointHit{
			Type: protocol.KindGraphBreakpointHit, GraphID: "g1", NodeID: "n1", EntityID: 99, Frame: frame,
		}},
		{protocol.KindGraphExecutionTrace, protocol.GraphExecutionTraceEvent{
			Type: protocol.KindGraphExecutionTrace, GraphID: "g1", EntityID: 99, Frames: []protocol.GraphExecutionFrame{frame},
		}},
		{protocol.KindGraphRuntimeError, protocol.GraphRuntimeError{
			Type: protocol.KindGraphRuntimeError, GraphID: "g1", NodeID: "n1", EntityID: 99,
			Error: "divide by zero", PinValues: map[string]any{"x": float64(0)},
		}},
		{protocol.KindGraphLiveUpdate, protocol.GraphLiveUpdate{
			Type: protocol.KindGraphLiveUpdate, GraphID: "g1",
			ChangeType: protocol.GraphConnectionAdded, Payload: map[string]any{"from": "n1", "to": "n2"},
		}},
	}
}

func TestGraphMessagesRoundTrip(t *testing.T) {
	for _, tc := range graphCases() {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			if err := protocol.Encode(&buf, tc.msg); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			got, kind, err := protocol.Decode(bufio.NewReader(&buf))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if kind != tc.kind {
				t.Errorf("kind = %q, want %q", kind, tc.kind)
			}
			wantJSON, _ := json.Marshal(tc.msg)
			gotJSON, _ := json.Marshal(got)
			if string(wantJSON) != string(gotJSON) {
				t.Errorf("round-trip mismatch:\nwant %s\n got %s", wantJSON, gotJSON)
			}
		})
	}
}

func TestGraphMessagesScanner(t *testing.T) {
	var buf bytes.Buffer
	cases := graphCases()
	for _, tc := range cases {
		if err := protocol.Encode(&buf, tc.msg); err != nil {
			t.Fatalf("Encode %v: %v", tc.kind, err)
		}
	}
	sc := protocol.NewScanner(&buf)
	n := 0
	for {
		_, _, ok, err := sc.Next()
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		if !ok {
			break
		}
		n++
	}
	if n != len(cases) {
		t.Errorf("scanned %d graph messages, want %d", n, len(cases))
	}
}

// TestGraphBreakpointDecodeConcrete confirms the decoded value is the concrete
// type with its nested frame intact.
func TestGraphBreakpointDecodeConcrete(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	src := protocol.GraphBreakpointHit{
		Type: protocol.KindGraphBreakpointHit, GraphID: "g", NodeID: "n", EntityID: 5,
		Frame: protocol.GraphExecutionFrame{NodeID: "n", NodeType: "flow.Branch", StepIndex: 3},
	}
	_ = protocol.Encode(&buf, src)
	got, _, err := protocol.Decode(bufio.NewReader(&buf))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	hit, ok := got.(protocol.GraphBreakpointHit)
	if !ok {
		t.Fatalf("decoded type = %T, want GraphBreakpointHit", got)
	}
	if hit.Frame.NodeType != "flow.Branch" || hit.Frame.StepIndex != 3 || hit.EntityID != 5 {
		t.Errorf("decoded = %+v", hit)
	}
}
