package material

import (
	"errors"
	"maps"
	"testing"

	pkgmath "github.com/neuengine/neu/pkg/math"
	gpu "github.com/neuengine/neu/pkg/render"
	"github.com/neuengine/neu/pkg/render/mesh"
)

// ─── AlphaMode → RenderPhase (INV-5) ──────────────────────────────────────────

func TestAlphaPhase(t *testing.T) {
	t.Parallel()
	tests := []struct {
		mode AlphaMode
		want gpu.RenderPhase
	}{
		{AlphaOpaque, gpu.PhaseOpaque},
		{AlphaMask, gpu.PhaseAlphaMask},
		{AlphaBlend, gpu.PhaseTransparent},
		{AlphaPremultiplied, gpu.PhaseTransparent},
		{AlphaAdditive, gpu.PhaseTransparent},
	}
	for _, tc := range tests {
		if got := tc.mode.Phase(); got != tc.want {
			t.Errorf("AlphaMode(%d).Phase() = %v, want %v", tc.mode, got, tc.want)
		}
	}
}

func TestAlphaPhaseHintBoundary(t *testing.T) {
	t.Parallel()
	// PhaseHint crossing the opaque/transparent boundary is silently rejected.
	tests := []struct {
		name  string
		alpha AlphaMode
		hint  gpu.RenderPhase
		want  gpu.RenderPhase
	}{
		{"opaque + transparent hint rejected", AlphaOpaque, gpu.PhaseTransparent, gpu.PhaseOpaque},
		{"blend + opaque hint rejected", AlphaBlend, gpu.PhaseOpaque, gpu.PhaseTransparent},
		{"mask + transparent hint rejected", AlphaMask, gpu.PhaseTransparent, gpu.PhaseAlphaMask},
		// Same-side hint is accepted.
		{"opaque + alphaMask hint accepted", AlphaOpaque, gpu.PhaseAlphaMask, gpu.PhaseAlphaMask},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hint := tc.hint
			m := &Material{Alpha: tc.alpha, PhaseHint: &hint}
			if got := m.resolvePhase(); got != tc.want {
				t.Errorf("resolvePhase() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ─── MaterialParameters.Sanitize (INV-2) ──────────────────────────────────────

func TestSanitize_Clamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		floats map[string]float32
		colors map[string]pkgmath.LinearRgba
		wantF  map[string]float32
		wantC  map[string]pkgmath.LinearRgba
		name   string
	}{
		{
			name:   "metallic over 1",
			floats: map[string]float32{"metallic": 1.5},
			wantF:  map[string]float32{"metallic": 1.0},
		},
		{
			name:   "roughness under 0",
			floats: map[string]float32{"roughness": -0.3},
			wantF:  map[string]float32{"roughness": 0.0},
		},
		{
			name:   "occlusion over 1",
			floats: map[string]float32{"occlusion": 2.0},
			wantF:  map[string]float32{"occlusion": 1.0},
		},
		{
			name:   "custom float not clamped",
			floats: map[string]float32{"custom": -5.0},
			wantF:  map[string]float32{"custom": -5.0},
		},
		{
			name:   "negative emissive clamped per component",
			colors: map[string]pkgmath.LinearRgba{"emissive": {R: -1, G: 2, B: -0.5, A: 1}},
			wantC:  map[string]pkgmath.LinearRgba{"emissive": {R: 0, G: 2, B: 0, A: 1}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := &MaterialParameters{Floats: tc.floats, Colors: tc.colors}
			p.Sanitize()
			for k, want := range tc.wantF {
				if got := p.Floats[k]; got != want {
					t.Errorf("Floats[%q] = %v, want %v", k, got, want)
				}
			}
			for k, want := range tc.wantC {
				if got := p.Colors[k]; got != want {
					t.Errorf("Colors[%q] = %v, want %v", k, got, want)
				}
			}
		})
	}
}

func TestSanitize_Idempotent(t *testing.T) {
	t.Parallel()
	p := &MaterialParameters{
		Floats: map[string]float32{"metallic": 1.8, "roughness": -0.2, "occlusion": 0.5},
		Colors: map[string]pkgmath.LinearRgba{"emissive": {R: -1, G: 0, B: 0, A: 1}},
	}
	p.Sanitize()

	// Deep-copy the post-sanitize state.
	snapF := make(map[string]float32, len(p.Floats))
	maps.Copy(snapF, p.Floats)
	snapC := make(map[string]pkgmath.LinearRgba, len(p.Colors))
	maps.Copy(snapC, p.Colors)

	p.Sanitize() // second call must be a no-op

	for k, want := range snapF {
		if got := p.Floats[k]; got != want {
			t.Errorf("Floats[%q] changed after second Sanitize: %v → %v", k, want, got)
		}
	}
	for k, want := range snapC {
		if got := p.Colors[k]; got != want {
			t.Errorf("Colors[%q] changed after second Sanitize: %v → %v", k, want, got)
		}
	}
}

func TestSanitize_NilMaps(t *testing.T) {
	t.Parallel()
	p := &MaterialParameters{} // nil Floats and Colors
	p.Sanitize()               // must not panic
}

// ─── Material.Validate (INV-1) ─────────────────────────────────────────────────

func TestMaterialValidate_NilShader(t *testing.T) {
	t.Parallel()
	m := &Material{} // zero Handle is weak → ErrMaterialNoShader
	if err := m.Validate(); !errors.Is(err, ErrMaterialNoShader) {
		t.Errorf("Validate() = %v, want ErrMaterialNoShader", err)
	}
}

func TestMaterialValidate_StandardPBR_NilShader(t *testing.T) {
	t.Parallel()
	// StandardPBR leaves Shader unset — still invalid until a shader is assigned.
	m := StandardPBR()
	if err := m.Validate(); !errors.Is(err, ErrMaterialNoShader) {
		t.Errorf("StandardPBR().Validate() = %v, want ErrMaterialNoShader", err)
	}
}

// ─── StandardPBR defaults ──────────────────────────────────────────────────────

func TestStandardPBR_Defaults(t *testing.T) {
	t.Parallel()
	m := StandardPBR()
	if m.Alpha != AlphaOpaque {
		t.Errorf("Alpha = %v, want AlphaOpaque", m.Alpha)
	}
	if got := m.Params.Floats["metallic"]; got != 0.0 {
		t.Errorf("metallic = %v, want 0.0", got)
	}
	if got := m.Params.Floats["roughness"]; got != 0.5 {
		t.Errorf("roughness = %v, want 0.5", got)
	}
	if got := m.Params.Colors["base_color"]; got != (pkgmath.LinearRgba{R: 1, G: 1, B: 1, A: 1}) {
		t.Errorf("base_color = %v, want white", got)
	}
}

func TestSetFloat_PBRClamped(t *testing.T) {
	t.Parallel()
	m := StandardPBR()
	m.SetFloat("metallic", 2.5)
	if got := m.Params.Floats["metallic"]; got != 1.0 {
		t.Errorf("metallic after SetFloat(2.5) = %v, want 1.0", got)
	}
	m.SetFloat("metallic", -1.0)
	if got := m.Params.Floats["metallic"]; got != 0.0 {
		t.Errorf("metallic after SetFloat(-1.0) = %v, want 0.0", got)
	}
}

func TestSetColor_NegativeClamped(t *testing.T) {
	t.Parallel()
	m := StandardPBR()
	m.SetColor("emissive", pkgmath.LinearRgba{R: -1, G: 1, B: -0.5, A: 1})
	want := pkgmath.LinearRgba{R: 0, G: 1, B: 0, A: 1}
	if got := m.Params.Colors["emissive"]; got != want {
		t.Errorf("emissive = %v, want %v", got, want)
	}
}

// ─── SpecializationKey ─────────────────────────────────────────────────────────

func TestSpecKey_Deterministic(t *testing.T) {
	t.Parallel()
	m := StandardPBR()
	l1, _ := buildTestLayouts()
	k1 := m.SpecKey(l1)
	k2 := m.SpecKey(l1)
	if k1 != k2 {
		t.Errorf("SpecKey not deterministic: %+v != %+v", k1, k2)
	}
}

func TestSpecKey_LayoutDifferentiates(t *testing.T) {
	t.Parallel()
	m := StandardPBR()
	l1, l2 := buildTestLayouts()
	if m.SpecKey(l1) == m.SpecKey(l2) {
		t.Error("different layouts produced identical SpecKey")
	}
}

func TestSpecKey_AlphaDifferentiates(t *testing.T) {
	t.Parallel()
	l, _ := buildTestLayouts()
	m1 := StandardPBR()
	m2 := StandardPBR()
	m2.Alpha = AlphaBlend
	if m1.SpecKey(l) == m2.SpecKey(l) {
		t.Error("different AlphaMode produced identical SpecKey")
	}
}

func BenchmarkSpecKey(b *testing.B) {
	m := StandardPBR()
	l, _ := buildTestLayouts()
	b.ReportAllocs()
	for b.Loop() {
		_ = m.SpecKey(l)
	}
}

// buildTestLayouts returns two distinct VertexLayouts for specialization-key tests.
func buildTestLayouts() (mesh.VertexLayout, mesh.VertexLayout) {
	// Layout 1: Position only (stride 12).
	m1 := mesh.NewMesh(mesh.TopologyTriangleList)
	m1.SetAttribute(mesh.VertexAttribute{
		Kind:   mesh.AttrPosition,
		Format: mesh.FormatFloat32x3,
		Data:   make([]byte, 12),
	})

	// Layout 2: Position + Normal (stride 24).
	m2 := mesh.NewMesh(mesh.TopologyTriangleList)
	m2.SetAttribute(mesh.VertexAttribute{
		Kind:   mesh.AttrPosition,
		Format: mesh.FormatFloat32x3,
		Data:   make([]byte, 12),
	})
	m2.SetAttribute(mesh.VertexAttribute{
		Kind:   mesh.AttrNormal,
		Format: mesh.FormatFloat32x3,
		Data:   make([]byte, 12),
	})

	return m1.Layout(), m2.Layout()
}
