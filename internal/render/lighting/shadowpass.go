package lighting

import (
	internalrender "github.com/neuengine/neu/internal/render"
	gpu "github.com/neuengine/neu/pkg/render"
)

// ShadowCaster describes a shadow-casting light for one frame.
// ShadowMapRID is the pre-allocated depth texture for this light's shadow map.
type ShadowCaster struct {
	ShadowMapRID gpu.RID
	Kind         LightKind
}

// shadowMapPass writes depth into one shadow map texture (INV-4: must execute
// before the lighting pass). Phase is PhaseNone — it is a pre-scene utility pass.
type shadowMapPass struct{ shadowRID gpu.RID }

func (p *shadowMapPass) Name() string                          { return "shadow_map" }
func (p *shadowMapPass) Phase() gpu.RenderPhase                { return gpu.PhaseNone }
func (p *shadowMapPass) Inputs() []gpu.RID                     { return nil }
func (p *shadowMapPass) Outputs() []gpu.RID                    { return []gpu.RID{p.shadowRID} }
func (p *shadowMapPass) Execute(_ *internalrender.PassContext) {}

// lightingPass reads all shadow maps and performs the clustered lighting shading.
// Its Inputs list is the union of all shadow map RIDs; the graph derives one
// producer→consumer edge per RID, enforcing shadow Before lighting (INV-4).
type lightingPass struct{ inputs []gpu.RID }

func (p *lightingPass) Name() string                          { return "lighting" }
func (p *lightingPass) Phase() gpu.RenderPhase                { return gpu.PhaseOpaque }
func (p *lightingPass) Inputs() []gpu.RID                     { return p.inputs }
func (p *lightingPass) Outputs() []gpu.RID                    { return nil }
func (p *lightingPass) Execute(_ *internalrender.PassContext) {}

// BuildShadowPasses registers one shadowMapPass per caster plus one lightingPass
// that reads all shadow maps into g. The shared RIDs create shadow→lighting graph
// edges, which topological sort enforces as shadow Before lighting (INV-4).
//
// Pass indices in the resulting graph:
//   - Caster i → index i (shadow pass)
//   - Lighting pass → index len(casters)
//
// Returns immediately (no-op) when casters is empty.
func BuildShadowPasses(g *internalrender.RenderGraph, casters []ShadowCaster) {
	if len(casters) == 0 {
		return
	}
	shadowRIDs := make([]gpu.RID, len(casters))
	for i, c := range casters {
		shadowRIDs[i] = c.ShadowMapRID
		g.AddPass(&shadowMapPass{shadowRID: c.ShadowMapRID})
	}
	g.AddPass(&lightingPass{inputs: shadowRIDs})
}
