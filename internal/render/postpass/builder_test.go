package postpass

import (
	"errors"
	"reflect"
	"testing"

	internalrender "github.com/neuengine/neu/internal/render"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/postprocess"
)

var sceneRID = gpu.MakeRID(gpu.KindTexture, 1, 0)

func buildGraph(slots []postprocess.EffectSlot) (*internalrender.RenderGraph, gpu.RID, error) {
	g := &internalrender.RenderGraph{}
	out, err := BuildPostChain(slots, g, sceneRID)
	if err != nil {
		return g, out, err
	}
	if err2 := g.Build(nil); err2 != nil {
		return g, out, err2
	}
	return g, out, nil
}

// ─── INV-2: deterministic canonical order regardless of insertion order ────────

func TestBuildPostChain_CanonicalOrder(t *testing.T) {
	t.Parallel()
	// Three permutations of the same slots must produce identical barrier lists.
	perms := [][]postprocess.EffectSlot{
		{postprocess.SlotSpatialAA, postprocess.SlotBloom, postprocess.SlotTonemapping},
		{postprocess.SlotBloom, postprocess.SlotTonemapping, postprocess.SlotSpatialAA},
		{postprocess.SlotTonemapping, postprocess.SlotSpatialAA, postprocess.SlotBloom},
	}
	var ref []internalrender.Barrier
	for i, p := range perms {
		g, _, err := buildGraph(p)
		if err != nil {
			t.Fatalf("perm %d: %v", i, err)
		}
		b := g.Barriers()
		if i == 0 {
			ref = b
		} else if !reflect.DeepEqual(ref, b) {
			t.Errorf("perm %d: barriers differ\n ref=%v\n got=%v", i, ref, b)
		}
	}
}

// ─── INV-3: disabled effects are absent; RIDs reconnect ───────────────────────

func TestBuildPostChain_OmitDisabled(t *testing.T) {
	t.Parallel()
	// With Bloom enabled: 3 passes, 2 barriers.
	g3, _, err := buildGraph([]postprocess.EffectSlot{
		postprocess.SlotBloom, postprocess.SlotTonemapping, postprocess.SlotSpatialAA,
	})
	if err != nil {
		t.Fatalf("3-pass build: %v", err)
	}
	if len(g3.Order()) != 3 {
		t.Errorf("3-pass: expected 3 nodes, got %d", len(g3.Order()))
	}
	if len(g3.Barriers()) != 2 {
		t.Errorf("3-pass: expected 2 barriers, got %d", len(g3.Barriers()))
	}

	// Without Bloom: 2 passes, 1 barrier; Bloom node removed, RIDs reconnect.
	g2, _, err := buildGraph([]postprocess.EffectSlot{
		postprocess.SlotTonemapping, postprocess.SlotSpatialAA,
	})
	if err != nil {
		t.Fatalf("2-pass build: %v", err)
	}
	if len(g2.Order()) != 2 {
		t.Errorf("2-pass: expected 2 nodes (Bloom omitted), got %d", len(g2.Order()))
	}
	if len(g2.Barriers()) != 1 {
		t.Errorf("2-pass: expected 1 barrier (Bloom removed), got %d", len(g2.Barriers()))
	}
}

// ─── INV-1: ErrPostOrder for HDR effect after tonemapping ─────────────────────

func TestValidateOrder_ErrPostOrder(t *testing.T) {
	t.Parallel()
	// Synthetic bad order: HDR slot (Bloom) after Tonemapping.
	err := validateOrder([]postprocess.EffectSlot{
		postprocess.SlotTonemapping, postprocess.SlotBloom,
	})
	if !errors.Is(err, postprocess.ErrPostOrder) {
		t.Errorf("expected ErrPostOrder, got %v", err)
	}
}

func TestValidateOrder_ValidOrders(t *testing.T) {
	t.Parallel()
	valid := [][]postprocess.EffectSlot{
		{postprocess.SlotBloom, postprocess.SlotTonemapping},
		{postprocess.SlotBloom, postprocess.SlotTonemapping, postprocess.SlotSpatialAA},
		{postprocess.SlotTonemapping},
		{postprocess.SlotSpatialAA},
		nil,
	}
	for _, v := range valid {
		if err := validateOrder(v); err != nil {
			t.Errorf("validateOrder(%v): unexpected error %v", v, err)
		}
	}
}

// ─── Empty slot list ──────────────────────────────────────────────────────────

func TestBuildPostChain_Empty(t *testing.T) {
	t.Parallel()
	g, out, err := buildGraph(nil)
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	// No slots → passthrough: output == sceneRID.
	if out != sceneRID {
		t.Errorf("empty: out = %v, want sceneRID", out)
	}
	if len(g.Order()) != 0 {
		t.Errorf("empty: expected 0 passes, got %d", len(g.Order()))
	}
}

// ─── Output RID is chained ────────────────────────────────────────────────────

func TestBuildPostChain_FinalOutputDiffersFromScene(t *testing.T) {
	t.Parallel()
	_, out, err := buildGraph([]postprocess.EffectSlot{postprocess.SlotTonemapping})
	if err != nil {
		t.Fatalf("single-pass: %v", err)
	}
	if out == sceneRID {
		t.Error("output RID should differ from sceneColor after at least one pass")
	}
}
