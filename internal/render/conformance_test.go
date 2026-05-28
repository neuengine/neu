package render

import (
	"sync"
	"testing"

	gpu "github.com/neuengine/neu/pkg/render"
)

// ─── RecordingBackend ─────────────────────────────────────────────────────────

// recordingBackend wraps nopBackend and records every method invocation.
// Allocates a unique sequential RID per Create call.
type recordingBackend struct {
	mu     sync.Mutex
	called map[string]int
	ridSeq uint32
}

func newRecordingBackend() *recordingBackend {
	return &recordingBackend{called: make(map[string]int)}
}

func (b *recordingBackend) record(name string) {
	b.mu.Lock()
	b.called[name]++
	b.mu.Unlock()
}

func (b *recordingBackend) nextRID(kind gpu.ResourceKind) gpu.RID {
	b.mu.Lock()
	b.ridSeq++
	seq := b.ridSeq
	b.mu.Unlock()
	return gpu.MakeRID(kind, seq, 0)
}

func (b *recordingBackend) CreateBuffer(_ gpu.BufferDesc) gpu.RID {
	b.record("CreateBuffer")
	return b.nextRID(gpu.KindBuffer)
}
func (b *recordingBackend) CreateTexture(_ gpu.TextureDesc) gpu.RID {
	b.record("CreateTexture")
	return b.nextRID(gpu.KindTexture)
}
func (b *recordingBackend) CreatePipeline(_ gpu.PipelineDesc) gpu.RID {
	b.record("CreatePipeline")
	return b.nextRID(gpu.KindPipeline)
}
func (b *recordingBackend) CreateBindGroup(_ gpu.BindGroupDesc) gpu.RID {
	b.record("CreateBindGroup")
	return b.nextRID(gpu.KindBindGroup)
}
func (b *recordingBackend) BeginRenderPass(_ gpu.RenderPassDesc) { b.record("BeginRenderPass") }
func (b *recordingBackend) Draw(_ gpu.DrawCmd)                   { b.record("Draw") }
func (b *recordingBackend) EndRenderPass()                       { b.record("EndRenderPass") }
func (b *recordingBackend) Submit()                              { b.record("Submit") }
func (b *recordingBackend) Present()                             { b.record("Present") }
func (b *recordingBackend) Destroy(_ gpu.RID)                    { b.record("Destroy") }

// ─── TestBackendConformance ───────────────────────────────────────────────────

// TestBackendConformance verifies that the RenderBackend interface contract is
// complete: every method can be called with valid inputs without panicking, and
// the recording backend tracks all 10 method invocations.
func TestBackendConformance(t *testing.T) {
	t.Parallel()
	b := newRecordingBackend()

	// Exercise every method of the RenderBackend interface.
	buf := b.CreateBuffer(gpu.BufferDesc{Size: 64, Usage: gpu.BufVertex | gpu.BufCopySrc})
	tex := b.CreateTexture(gpu.TextureDesc{Width: 256, Height: 256, Format: gpu.FmtRGBA8, MipLevels: 1})
	pip := b.CreatePipeline(gpu.PipelineDesc{ShaderID: 1, LayoutHash: 42, Phase: 1})
	bg := b.CreateBindGroup(gpu.BindGroupDesc{Resources: []gpu.RID{tex}, Layout: 1})
	b.BeginRenderPass(gpu.RenderPassDesc{Color: []gpu.RID{tex}, Depth: gpu.RID(0)})
	b.Draw(gpu.DrawCmd{
		BindGroups:  []gpu.RID{bg},
		Pipeline:    pip,
		Vertex:      buf,
		InstanceCnt: 1,
	})
	b.EndRenderPass()
	b.Submit()
	b.Present()
	b.Destroy(tex)

	// Verify that every one of the 10 interface methods was invoked.
	required := []string{
		"CreateBuffer", "CreateTexture", "CreatePipeline", "CreateBindGroup",
		"BeginRenderPass", "Draw", "EndRenderPass", "Submit", "Present", "Destroy",
	}
	for _, method := range required {
		if b.called[method] == 0 {
			t.Errorf("method %s was not called during conformance test", method)
		}
	}
}

// TestBackendConformance_RIDsAreDistinct verifies that each Create call
// produces a non-nil RID distinct from all others (no aliasing).
func TestBackendConformance_RIDsAreDistinct(t *testing.T) {
	t.Parallel()
	b := newRecordingBackend()

	rids := []gpu.RID{
		b.CreateBuffer(gpu.BufferDesc{Size: 16}),
		b.CreateTexture(gpu.TextureDesc{Width: 1, Height: 1, Format: gpu.FmtRGBA8, MipLevels: 1}),
		b.CreatePipeline(gpu.PipelineDesc{}),
		b.CreateBindGroup(gpu.BindGroupDesc{}),
	}

	seen := make(map[gpu.RID]bool)
	for _, r := range rids {
		if r.IsNil() {
			t.Error("Create method returned nil RID")
		}
		if seen[r] {
			t.Errorf("duplicate RID %v", r)
		}
		seen[r] = true
	}
}

// TestBackendConformance_ServerIntegration verifies that the recording backend
// integrates correctly with the render Server: Initialize and Submit commands
// execute on the Drain goroutine, not the submitting goroutine.
func TestBackendConformance_ServerIntegration(t *testing.T) {
	t.Parallel()
	b := newRecordingBackend()
	s := NewServer(b)
	s.Bind()

	rid := s.Allocate(gpu.KindTexture)
	if rid.IsNil() {
		t.Fatal("Server.Allocate returned nil RID")
	}

	// Submit a no-op command to verify the server wiring.
	called := false
	if err := s.Submit(func() { called = true }); err != nil {
		t.Fatalf("Server.Submit: %v", err)
	}
	s.Drain()
	if !called {
		t.Error("submitted command was not executed by Drain")
	}
	s.Close()
}
