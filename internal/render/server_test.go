package render

import (
	"sync"
	"sync/atomic"
	"testing"

	gpu "github.com/neuengine/neu/pkg/render"
)

// nopBackend is a no-op RenderBackend for server tests (no resource tracking).
type nopBackend struct{}

func (nopBackend) CreateBuffer(gpu.BufferDesc) gpu.RID       { return 0 }
func (nopBackend) CreateTexture(gpu.TextureDesc) gpu.RID     { return 0 }
func (nopBackend) CreatePipeline(gpu.PipelineDesc) gpu.RID   { return 0 }
func (nopBackend) CreateBindGroup(gpu.BindGroupDesc) gpu.RID { return 0 }
func (nopBackend) BeginRenderPass(gpu.RenderPassDesc)        {}
func (nopBackend) Draw(gpu.DrawCmd)                          {}
func (nopBackend) EndRenderPass()                            {}
func (nopBackend) Submit()                                   {}
func (nopBackend) Present()                                  {}
func (nopBackend) Destroy(gpu.RID)                           {}

func TestServer_AllocateUniqueRIDs(t *testing.T) {
	s := NewServer(nopBackend{})
	seen := make(map[gpu.RID]bool)
	for range 1000 {
		r := s.Allocate(gpu.KindTexture)
		if r.Kind() != gpu.KindTexture {
			t.Fatalf("Kind = %d, want KindTexture", r.Kind())
		}
		if r.IsNil() {
			t.Fatal("Allocate returned nil RID")
		}
		if seen[r] {
			t.Fatalf("duplicate RID %d", r)
		}
		seen[r] = true
	}
}

// TestServer_ConcurrentSubmitDrain verifies that commands submitted from many
// goroutines are dedup-run exactly once, all on the bound (Drain) goroutine.
// Run under -race (C-005).
func TestServer_ConcurrentSubmitDrain(t *testing.T) {
	s := NewServer(nopBackend{})
	s.Bind() // this test goroutine is the server goroutine
	wantGID := currentGID()

	const n = 256
	var ran atomic.Int64
	var wrongGoroutine atomic.Bool
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.Submit(func() {
				if currentGID() != wantGID {
					wrongGoroutine.Store(true)
				}
				ran.Add(1)
			}); err != nil {
				t.Errorf("Submit: %v", err)
			}
		}()
	}
	wg.Wait()
	s.Drain()

	if got := ran.Load(); got != n {
		t.Fatalf("ran %d commands, want %d", got, n)
	}
	if wrongGoroutine.Load() {
		t.Fatal("a command ran off the bound server goroutine")
	}
}

// TestServer_InlineFIFO verifies an inline submission from the server goroutine
// runs after everything already queued by other goroutines (global FIFO).
func TestServer_InlineFIFO(t *testing.T) {
	s := NewServer(nopBackend{})
	s.Bind()

	var order []int
	done := make(chan struct{})
	go func() {
		_ = s.Submit(func() { order = append(order, 1) })
		close(done)
	}()
	<-done
	// Inline submission from the bound goroutine: must drain the queued (1)
	// before running this (2).
	if err := s.Submit(func() { order = append(order, 2) }); err != nil {
		t.Fatalf("inline Submit: %v", err)
	}
	if len(order) != 2 || order[0] != 1 || order[1] != 2 {
		t.Fatalf("order = %v, want [1 2] (global FIFO violated)", order)
	}
}

func TestServer_SubmitAfterClose(t *testing.T) {
	s := NewServer(nopBackend{})
	s.Close()
	if err := s.Submit(func() {}); err != ErrRenderClosed {
		t.Fatalf("Submit after Close = %v, want ErrRenderClosed", err)
	}
	if err := s.Initialize(s.Allocate(gpu.KindBuffer), func(gpu.RenderBackend) {}); err != ErrRenderClosed {
		t.Fatalf("Initialize after Close = %v, want ErrRenderClosed", err)
	}
}

// TestServer_TwoPhaseCreate: Allocate is synchronous; Initialize is deferred
// and only runs on Drain, receiving the backend.
func TestServer_TwoPhaseCreate(t *testing.T) {
	s := NewServer(nopBackend{})
	rid := s.Allocate(gpu.KindBuffer) // immediate
	if rid.IsNil() {
		t.Fatal("Allocate returned nil")
	}
	var initialized bool
	if err := s.Initialize(rid, func(b gpu.RenderBackend) {
		if b == nil {
			t.Error("Initialize got nil backend")
		}
		initialized = true
	}); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	if initialized {
		t.Fatal("Initialize ran synchronously; must be deferred to Drain")
	}
	s.Drain()
	if !initialized {
		t.Fatal("Initialize did not run on Drain")
	}
}
