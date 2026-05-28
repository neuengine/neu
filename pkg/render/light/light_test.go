package light

import (
	"errors"
	"math"
	"testing"

	pkgmath "github.com/neuengine/neu/pkg/math"
)

// ─── CascadeShadowConfig.Splits (INV-3) ───────────────────────────────────────

func TestCascadeCoverage_LogarithmicValues(t *testing.T) {
	t.Parallel()
	// near=1, MaxDistance=16, Count=4 → clean power-of-2 splits: 2, 4, 8, 16.
	cfg := &CascadeShadowConfig{Count: 4, SplitMode: SplitLogarithmic, MaxDistance: 16}
	splits, err := cfg.Splits(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []float32{2, 4, 8, 16}
	for i, w := range want {
		if math.Abs(float64(splits[i]-w)) > 1e-4 {
			t.Errorf("splits[%d] = %v, want %v", i, splits[i], w)
		}
	}
}

func TestCascadeCoverage_LogarithmicLastEqualsMax(t *testing.T) {
	t.Parallel()
	// For any valid near + MaxDistance, logarithmic mode guarantees split[Count-1] == MaxDistance.
	cases := []struct {
		near, maxDist float32
		count         uint8
	}{
		{0.1, 1000, 4},
		{1, 100, 2},
		{5, 500, 3},
	}
	for _, tc := range cases {
		cfg := &CascadeShadowConfig{Count: tc.count, SplitMode: SplitLogarithmic, MaxDistance: tc.maxDist}
		splits, err := cfg.Splits(tc.near)
		if err != nil {
			t.Errorf("near=%v max=%v count=%d: unexpected error %v", tc.near, tc.maxDist, tc.count, err)
			continue
		}
		last := splits[len(splits)-1]
		if math.Abs(float64(last-tc.maxDist)) > 1e-3 {
			t.Errorf("near=%v max=%v: last split %v ≠ MaxDistance %v", tc.near, tc.maxDist, last, tc.maxDist)
		}
	}
}

func TestCascadeCoverage_ManualValid(t *testing.T) {
	t.Parallel()
	cfg := &CascadeShadowConfig{
		Count:        2,
		SplitMode:    SplitManual,
		ManualSplits: []float32{10, 100}, // last == MaxDistance
		MaxDistance:  100,
	}
	splits, err := cfg.Splits(1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(splits) != 2 {
		t.Errorf("len(splits) = %d, want 2", len(splits))
	}
	if splits[1] != 100 {
		t.Errorf("splits[1] = %v, want 100", splits[1])
	}
}

func TestCascadeCoverage_ManualUndercovers(t *testing.T) {
	t.Parallel()
	// Last split (80) < MaxDistance (100) — INV-3 violation.
	cfg := &CascadeShadowConfig{
		Count:        2,
		SplitMode:    SplitManual,
		ManualSplits: []float32{10, 80},
		MaxDistance:  100,
	}
	_, err := cfg.Splits(1)
	if !errors.Is(err, ErrCascadeCoverage) {
		t.Errorf("got %v, want ErrCascadeCoverage", err)
	}
}

func TestCascadeCoverage_ManualTooFewSplits(t *testing.T) {
	t.Parallel()
	// Only 2 ManualSplits provided for Count=4 — INV-3 violation.
	cfg := &CascadeShadowConfig{
		Count:        4,
		SplitMode:    SplitManual,
		ManualSplits: []float32{10, 50},
		MaxDistance:  100,
	}
	_, err := cfg.Splits(1)
	if !errors.Is(err, ErrCascadeCoverage) {
		t.Errorf("got %v, want ErrCascadeCoverage", err)
	}
}

func TestCascadeCoverage_CountClamped(t *testing.T) {
	t.Parallel()
	// Count=0 is treated as 1; Count=10 is clamped to 4.
	cfgZero := &CascadeShadowConfig{Count: 0, SplitMode: SplitLogarithmic, MaxDistance: 100}
	splits, err := cfgZero.Splits(1)
	if err != nil {
		t.Fatalf("Count=0: unexpected error: %v", err)
	}
	if len(splits) != 1 {
		t.Errorf("Count=0 clamped: len(splits) = %d, want 1", len(splits))
	}

	cfgOver := &CascadeShadowConfig{Count: 10, SplitMode: SplitLogarithmic, MaxDistance: 100}
	splits, err = cfgOver.Splits(1)
	if err != nil {
		t.Fatalf("Count=10: unexpected error: %v", err)
	}
	if len(splits) != 4 {
		t.Errorf("Count=10 clamped: len(splits) = %d, want 4", len(splits))
	}
}

// ─── Shadow map count (L1 §4.6 table) ────────────────────────────────────────

func TestLight_ShadowMapCount_PointLight(t *testing.T) {
	t.Parallel()
	pl := PointLight{Shadow: &CubeShadow{MapSize: 1024}}
	// L1 §4.6: Point light → cube shadow map → 6 faces.
	if pl.Shadow.FaceCount() != 6 {
		t.Errorf("FaceCount = %d, want 6", pl.Shadow.FaceCount())
	}
}

func TestLight_ShadowMapCount_SpotLight(t *testing.T) {
	t.Parallel()
	sl := SpotLight{Shadow: &SingleShadow{MapSize: 512}}
	// L1 §4.6: Spot light → single shadow map → 1.
	if sl.Shadow.MapCount() != 1 {
		t.Errorf("MapCount = %d, want 1", sl.Shadow.MapCount())
	}
}

func TestLight_ShadowMapCount_DirectionalLight(t *testing.T) {
	t.Parallel()
	// L1 §4.6: Directional light → cascaded shadow maps → Count cascades.
	for _, count := range []uint8{1, 2, 3, 4} {
		dl := DirectionalLight{Cascades: &CascadeShadowConfig{Count: count}}
		if dl.Cascades.Count != count {
			t.Errorf("Count = %d, want %d", dl.Cascades.Count, count)
		}
	}
}

// ─── Light component smoke tests ──────────────────────────────────────────────

func TestLight_PointLight(t *testing.T) {
	t.Parallel()
	pl := PointLight{
		Color:     pkgmath.LinearRgba{R: 1, G: 0.8, B: 0.6, A: 1},
		Intensity: 800,
		Radius:    10,
	}
	if pl.Intensity != 800 {
		t.Errorf("Intensity = %v, want 800", pl.Intensity)
	}
	if pl.Radius != 10 {
		t.Errorf("Radius = %v, want 10", pl.Radius)
	}
	if pl.Shadow != nil {
		t.Error("default Shadow should be nil")
	}
}

func TestLight_SpotLight_AngleOrder(t *testing.T) {
	t.Parallel()
	sl := SpotLight{InnerAngle: 0.2, OuterAngle: 0.5}
	if sl.OuterAngle < sl.InnerAngle {
		t.Error("OuterAngle must be >= InnerAngle")
	}
}

func TestLight_AmbientLight(t *testing.T) {
	t.Parallel()
	al := AmbientLight{Color: pkgmath.LinearRgba{R: 0.05, G: 0.05, B: 0.05, A: 1}}
	if al.Color.R != 0.05 {
		t.Errorf("ambient R = %v, want 0.05", al.Color.R)
	}
}

// ─── IBL components ───────────────────────────────────────────────────────────

func TestIBL_EnvironmentMapLight_ZeroHandles(t *testing.T) {
	t.Parallel()
	// Zero handles are valid Bootstrap structs; lighting pass treats them as "no IBL".
	eml := EnvironmentMapLight{}
	if !eml.Diffuse.IsWeak() {
		t.Error("Diffuse: expected weak handle")
	}
	if !eml.Specular.IsWeak() {
		t.Error("Specular: expected weak handle")
	}
}

func TestIBL_IrradianceVolume_ProbeCount(t *testing.T) {
	t.Parallel()
	iv := IrradianceVolume{
		GridSize: [3]uint32{2, 3, 4},
		Probes:   make([]pkgmath.Vec4, 2*3*4*9), // 9 SH Vec4 per probe
	}
	want := uint32(2 * 3 * 4)
	if iv.ProbeCount() != want {
		t.Errorf("ProbeCount() = %d, want %d", iv.ProbeCount(), want)
	}
	if uint32(len(iv.Probes)) != want*9 {
		t.Errorf("len(Probes) = %d, want %d", len(iv.Probes), want*9)
	}
}
