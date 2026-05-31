//go:build editor

package assistant

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStdioConnection(t *testing.T) {
	t.Parallel()
	// Inbound: one newline-framed message the agent "sent".
	inbound, _ := Encode(AgentMessage{ID: "9", Type: MsgResponse, Result: "pong"})
	var outbound bytes.Buffer
	conn := NewStdioConnection(&outbound, strings.NewReader(string(inbound)))

	if !conn.IsAlive() {
		t.Fatal("new connection should be alive")
	}
	// Send writes a framed message to the writer.
	if err := conn.Send(context.Background(), AgentMessage{ID: "9", Type: MsgRequest, Method: "chat"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.HasSuffix(outbound.String(), "\n") || !strings.Contains(outbound.String(), "chat") {
		t.Errorf("Send output = %q", outbound.String())
	}
	// Receive decodes the inbound message.
	got, err := conn.Receive(context.Background())
	if err != nil || got.Result != "pong" {
		t.Errorf("Receive = %+v, %v", got, err)
	}
	// Close ⇒ not alive ⇒ Send fails.
	conn.Close()
	if conn.IsAlive() {
		t.Error("closed connection should not be alive")
	}
	if err := conn.Send(context.Background(), AgentMessage{}); !errors.Is(err, ErrConnClosed) {
		t.Errorf("Send after close = %v, want ErrConnClosed", err)
	}
	// A cancelled context short-circuits Send.
	cctx, cancel := context.WithCancel(context.Background())
	cancel()
	live := NewStdioConnection(&outbound, strings.NewReader(""))
	if err := live.Send(cctx, AgentMessage{}); err == nil {
		t.Error("cancelled context Send should error")
	}
}

func TestCapabilityBitfield(t *testing.T) {
	t.Parallel()
	c := ReadScenes.With(WriteScenes)
	if !c.Has(ReadScenes) || !c.Has(WriteScenes) {
		t.Error("With should add capabilities")
	}
	if c.Has(CodeGeneration) {
		t.Error("ungranted capability present")
	}
	if Capability(0).String() != "none" {
		t.Error("empty caps String")
	}
	if (ReadScenes | WriteScenes).String() != "ReadScenes|WriteScenes" {
		t.Errorf("String = %q", (ReadScenes | WriteScenes).String())
	}
}

func TestRequiredCapability(t *testing.T) {
	t.Parallel()
	if RequiredCapability(MethodGenerateScene) != (ReadTypeRegistry | WriteScenes) {
		t.Error("generate_scene caps")
	}
	if RequiredCapability(MethodChat) != ReadTypeRegistry {
		t.Error("chat caps")
	}
	// Custom method → conservative default.
	if RequiredCapability("com.example.custom") != ExecuteCommands {
		t.Error("custom method should default to ExecuteCommands")
	}
	if !IsStandardMethod(MethodChat) || IsStandardMethod("custom") {
		t.Error("IsStandardMethod")
	}
}

func TestMessageRoundTrip(t *testing.T) {
	t.Parallel()
	m := AgentMessage{ID: "1", Type: MsgRequest, Method: "chat", Params: map[string]any{"prompt": "hi"}}
	b, err := Encode(m)
	if err != nil || b[len(b)-1] != '\n' {
		t.Fatalf("Encode = %q, %v", b, err)
	}
	got, err := Decode(b[:len(b)-1])
	if err != nil || got.Method != "chat" || got.Params["prompt"] != "hi" {
		t.Errorf("Decode = %+v, %v", got, err)
	}
}

func okResponder(_ context.Context, req AgentMessage) (AgentMessage, error) {
	return AgentMessage{ID: req.ID, Type: MsgResponse, Result: "ok"}, nil
}

func TestDispatchCapabilityGate(t *testing.T) {
	t.Parallel()
	m := NewAssistantManager(time.Second)
	// Agent granted only ReadTypeRegistry — cannot generate_scene (needs WriteScenes).
	m.RegisterAgent("a1", NewMemConnection(okResponder), ReadTypeRegistry)

	_, err := m.Dispatch(context.Background(), "a1", MethodGenerateScene, nil)
	if !errors.Is(err, ErrCapabilityDenied) {
		t.Errorf("INV-2: generate_scene without WriteScenes = %v, want ErrCapabilityDenied", err)
	}
	// chat (ReadTypeRegistry) succeeds.
	resp, err := m.Dispatch(context.Background(), "a1", MethodChat, nil)
	if err != nil || resp.Result != "ok" {
		t.Errorf("chat dispatch = %+v, %v", resp, err)
	}
	// Log records both (one denied, one ok).
	log := m.RequestLog()
	if len(log) != 2 || log[0].OK || !log[1].OK {
		t.Errorf("request log = %+v", log)
	}
}

func TestDispatchUnknownAgent(t *testing.T) {
	t.Parallel()
	m := NewAssistantManager(time.Second)
	if _, err := m.Dispatch(context.Background(), "ghost", MethodChat, nil); !errors.Is(err, ErrUnknownAgent) {
		t.Errorf("unknown agent err = %v", err)
	}
}

func TestDispatchTimeout(t *testing.T) {
	t.Parallel()
	// A responder that blocks until the context is cancelled simulates a dead agent.
	blocking := func(ctx context.Context, _ AgentMessage) (AgentMessage, error) {
		<-ctx.Done()
		return AgentMessage{}, ctx.Err()
	}
	m := NewAssistantManager(50 * time.Millisecond)
	m.RegisterAgent("slow", NewMemConnection(blocking), ReadTypeRegistry)

	start := time.Now()
	_, err := m.Dispatch(context.Background(), "slow", MethodChat, nil)
	if !errors.Is(err, ErrAgentUnavailable) {
		t.Errorf("INV-5: slow agent = %v, want ErrAgentUnavailable", err)
	}
	if time.Since(start) > time.Second {
		t.Error("INV-5: dispatch did not respect the timeout")
	}
}

// recordingModifier captures applied modifications for INV-1/4 verification.
type recordingModifier struct{ ops []Modification }

func (r *recordingModifier) Apply(mod Modification) { r.ops = append(r.ops, mod) }

func TestApplyModificationsTagging(t *testing.T) {
	t.Parallel()
	m := NewAssistantManager(time.Second)
	var rec recordingModifier
	ops := []Modification{{Op: "spawn"}, {Op: "insert"}}
	m.ApplyModifications(&rec, "a1", "req-7", ops)

	if len(rec.ops) != 2 {
		t.Fatalf("applied %d ops, want 2", len(rec.ops))
	}
	for _, op := range rec.ops {
		if op.Agent != "a1" || op.Request != "req-7" {
			t.Errorf("INV-4: op not tagged with agent+request: %+v", op)
		}
	}
}

func TestAgentCount(t *testing.T) {
	t.Parallel()
	m := NewAssistantManager(0) // default timeout
	m.RegisterAgent("a", NewMemConnection(nil), 0)
	m.RegisterAgent("b", NewMemConnection(nil), 0)
	if m.AgentCount() != 2 {
		t.Errorf("AgentCount = %d, want 2", m.AgentCount())
	}
}
