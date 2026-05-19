package render

import (
	"github.com/neuengine/neu/internal/ecs/world"
	gpu "github.com/neuengine/neu/pkg/render"
)

// RenderFeature is a complete render capability (MeshRendering, ShadowMapping,
// PostProcessing, …) that participates in every pipeline stage
// (l1-render-core §4.10). Features own their data and create lightweight
// [RenderObject] proxies during Extract — they do NOT pollute ECS components
// with GPU state, and RenderObjects are NOT ECS entities.
//
// Bootstrap 0.1.0: concrete features arrive in Tracks B–E (mesh, materials,
// camera, post). This is the contract + dispatch wiring only.
type RenderFeature interface {
	Initialize(*RenderSubApp)
	Collect(*CollectContext)
	Extract(*ExtractContext)
	PrepareEffectPermutations(*PrepareContext)
	Prepare(*PrepareContext)
	Draw(ctx *DrawContext, view *RenderView)
	Flush(*FlushContext)
}

// Per-stage contexts carry the render world, frame counter, SoA holder, and
// (Draw) the command server. Minimal in 0.1.0 — fields grow with features.
type (
	CollectContext struct {
		Frame  uint64
		Render *world.World
	}
	ExtractContext struct {
		Frame  uint64
		Main   *world.World
		Render *world.World
		Data   *RenderDataHolder
	}
	PrepareContext struct {
		Frame  uint64
		Render *world.World
		Data   *RenderDataHolder
		Server *Server
	}
	DrawContext struct {
		Frame   uint64
		Backend gpu.RenderBackend
		Server  *Server
		Graph   *RenderGraph // features contribute passes here
	}
	FlushContext struct {
		Frame uint64
	}
)
