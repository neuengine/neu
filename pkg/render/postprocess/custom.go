package postprocess

import (
	"github.com/neuengine/neu/pkg/asset"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/material"
)

// ShaderValue is a typed shader parameter value for FullscreenMaterial.
// Concrete types: float32, [4]float32 (vec4), gpu.RID (texture).
type ShaderValue = any

// FullscreenMaterial is a user-defined fullscreen render pass injected into
// the canonical post-process chain after a specific EffectSlot (§4.5).
//
// The pass reads from the current intermediate texture (auto-wired by the
// builder) and writes to the next. Params are bound as shader constants.
// Compile failure → node omitted, chain reconnects (per Error Handling table).
type FullscreenMaterial struct {
	Shader      asset.Handle[material.Shader]
	InputTex    []gpu.RID
	Params      map[string]ShaderValue
	InsertAfter EffectSlot // inject the pass immediately after this slot
}
