package postpass

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/pkg/render/postprocess"
)

// ─── AA mutual-exclusion ──────────────────────────────────────────────────────

func TestCheckAAConflict_BothEnabled(t *testing.T) {
	t.Parallel()
	// FXAA + SMAA simultaneously → ErrAAConflict (SMAA preferred).
	err := CheckAAConflict(true, true)
	if !errors.Is(err, postprocess.ErrAAConflict) {
		t.Errorf("expected ErrAAConflict, got %v", err)
	}
}

func TestCheckAAConflict_SMAAOnly(t *testing.T) {
	t.Parallel()
	if err := CheckAAConflict(false, true); err != nil {
		t.Errorf("SMAA only: unexpected error %v", err)
	}
}

func TestCheckAAConflict_FXAAOnly(t *testing.T) {
	t.Parallel()
	if err := CheckAAConflict(true, false); err != nil {
		t.Errorf("FXAA only: unexpected error %v", err)
	}
}

func TestCheckAAConflict_NeitherEnabled(t *testing.T) {
	t.Parallel()
	if err := CheckAAConflict(false, false); err != nil {
		t.Errorf("neither AA: unexpected error %v", err)
	}
}

// ─── PingPongPool ─────────────────────────────────────────────────────────────

func TestPingPongPool_HDRLDRDistinct(t *testing.T) {
	t.Parallel()
	pool := NewPingPongPool()
	hdr := pool.AcquireHDR()
	ldr := pool.AcquireLDR()
	if hdr == ldr {
		t.Error("HDR and LDR targets must be distinct RIDs")
	}
}

func TestPingPongPool_ResetRestoresTargets(t *testing.T) {
	t.Parallel()
	pool := NewPingPongPool()
	first := pool.AcquireHDR()
	pool.Reset()
	second := pool.AcquireHDR()
	if first != second {
		t.Errorf("after Reset, AcquireHDR must return same RID: first=%v second=%v", first, second)
	}
}

func TestPingPongPool_AlternatesOnConsecutiveAcquire(t *testing.T) {
	t.Parallel()
	pool := NewPingPongPool()
	a := pool.AcquireHDR()
	b := pool.AcquireHDR()
	// Two consecutive HDR acquires must use different slots.
	if a == b {
		t.Error("two consecutive HDR acquires should return different ping-pong targets")
	}
	// Third acquire wraps back to the first slot.
	c := pool.AcquireHDR()
	if c != a {
		t.Errorf("third HDR acquire should wrap to first slot: got %v, want %v", c, a)
	}
}

// ─── INV-4: render-world isolation ───────────────────────────────────────────

// TestRenderWorldIsolation verifies that PostProcessStack satisfies INV-4:
// a copy (the "extracted" render-world snapshot) is unaffected by mutations
// applied to the original (main-world) stack after extraction.
func TestRenderWorldIsolation(t *testing.T) {
	t.Parallel()
	var main postprocess.PostProcessStack
	main.Enable(postprocess.SlotBloom).Enable(postprocess.SlotTonemapping)

	// Simulate Extract: copy to render world (value semantics = deep copy).
	snapshot := main

	// Mutate main world after extraction.
	main.Enable(postprocess.SlotSpatialAA)
	main.Disable(postprocess.SlotBloom)

	// Render-world snapshot must not reflect main-world mutations.
	if snapshot.IsEnabled(postprocess.SlotSpatialAA) {
		t.Error("snapshot saw SpatialAA enabled — render-world isolation violated")
	}
	if !snapshot.IsEnabled(postprocess.SlotBloom) {
		t.Error("snapshot lost Bloom — render-world isolation violated")
	}
}

// ─── BenchmarkBuildPostChain: pooled ping-pong steady-state ──────────────────

// BenchmarkBuildPostChain measures the per-frame steady-state cost of ping-pong
// target management (pool.Reset + acquire) — the allocation-free path that
// C-027 mandates. Graph rebuild is a one-time event on settings change and is
// not measured here (same reconciliation as BenchmarkFrustumCullSoA / C-027).
func BenchmarkBuildPostChain(b *testing.B) {
	pool := NewPingPongPool()
	pool.Reset() // warm up

	b.ResetTimer()
	b.ReportAllocs()
	for b.Loop() {
		pool.Reset()
		_ = pool.AcquireHDR() // Bloom (HDR)
		_ = pool.AcquireHDR() // Tonemapping (HDR→LDR boundary)
		_ = pool.AcquireLDR() // SpatialAA (LDR)
	}
}
