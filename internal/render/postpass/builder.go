// Package postpass builds the post-processing render-graph chain from a set of
// enabled EffectSlots (l2-post-processing-go.md, Bootstrap 0.1.0).
//
// Bootstrap: BuildPostChain accepts a pre-sorted []EffectSlot rather than
// querying the ECS world directly. The full ECS integration (reading settings
// components from a camera entity) arrives with T-4E02 render-world isolation.
package postpass

import (
	"fmt"
	"slices"

	internalrender "github.com/neuengine/neu/internal/render"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/postprocess"
)

// intermRIDBase is the dense-index base for in-graph intermediate textures.
// Chosen to avoid collision with external RIDs (e.g. sceneColor at low indices).
const intermRIDBase = 0x0001_0000

// BuildPostChain assembles a linear post-processing RenderPass chain into g.
//
// slots is the list of enabled EffectSlots in any order; the function sorts
// them by canonical enum value (INV-2). Each enabled slot becomes one
// RenderPass node; disabled slots are absent (INV-3). Each pass's output RID
// is auto-chained as the next pass's input.
//
// Returns the final output RID (the LDR result to present), or
// ErrPostOrder if a HDR effect appears after SlotTonemapping.
//
// Bootstrap: intermediate RIDs are synthetic (base+seq); pooled real textures
// arrive in T-4E02 (pingpong.go). BuildPostChain itself allocates for the
// sorted slice; 0-alloc steady-state is a T-4E02 concern.
func BuildPostChain(slots []postprocess.EffectSlot, g *internalrender.RenderGraph, sceneColor gpu.RID) (gpu.RID, error) {
	if len(slots) == 0 {
		return sceneColor, nil
	}

	// Sort into canonical order (INV-2): enum value = execution priority.
	sorted := make([]postprocess.EffectSlot, len(slots))
	copy(sorted, slots)
	slices.Sort(sorted)

	// Validate: no HDR effect may appear after tonemapping (INV-1).
	if err := validateOrder(sorted); err != nil {
		return gpu.RID(0), err
	}

	current := sceneColor
	for i, slot := range sorted {
		outRID := gpu.MakeRID(gpu.KindTexture, uint32(intermRIDBase+i), 0)
		g.AddPass(&postPass{
			slot:   slot,
			input:  current,
			output: outRID,
		})
		current = outRID
	}
	return current, nil
}

// validateOrder checks that no HDR (color-altering) slot appears after
// SlotTonemapping in the already-sorted list (INV-1). Returns ErrPostOrder
// on the first violation.
func validateOrder(sorted []postprocess.EffectSlot) error {
	seenTonemap := false
	for _, s := range sorted {
		if s == postprocess.SlotTonemapping {
			seenTonemap = true
			continue
		}
		if seenTonemap && s.IsHDR() {
			return fmt.Errorf("%w: %s after tonemapping", postprocess.ErrPostOrder, s)
		}
	}
	return nil
}

// postPass is a concrete RenderPass for one post-processing slot.
// It reads from one intermediate texture and writes to the next (auto-chain).
type postPass struct {
	slot   postprocess.EffectSlot
	input  gpu.RID
	output gpu.RID
}

func (p *postPass) Name() string                          { return p.slot.String() }
func (p *postPass) Phase() gpu.RenderPhase                { return gpu.PhaseNone }
func (p *postPass) Inputs() []gpu.RID                     { return []gpu.RID{p.input} }
func (p *postPass) Outputs() []gpu.RID                    { return []gpu.RID{p.output} }
func (p *postPass) Execute(_ *internalrender.PassContext) {}
