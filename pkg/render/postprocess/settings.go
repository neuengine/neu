package postprocess

// BokehShape selects the aperture shape for depth-of-field bokeh.
type BokehShape uint8

const (
	BokehCircle  BokehShape = iota
	BokehHexagon
)

// BloomSettings configures the bloom effect (SlotBloom, HDR).
type BloomSettings struct {
	Threshold   float32 // luminance threshold for bloom extraction
	Intensity   float32 // bloom contribution scale
	Knee        float32 // soft-knee width around threshold
	MaxMipLevel uint8   // caps the downsample pyramid depth
}

// SsaoSettings configures screen-space ambient occlusion (SlotSSAO, HDR).
type SsaoSettings struct {
	Radius      float32
	Bias        float32
	Intensity   float32
	SampleCount uint16
	GroundTruth bool // use GTAO variant when true
}

// DofSettings configures depth of field (SlotDepthOfField, HDR).
type DofSettings struct {
	FocalDistance float32
	FocalRange    float32
	MaxBlur       float32
	Bokeh         BokehShape
}

// MotionBlurSettings configures motion blur (SlotMotionBlur, HDR).
type MotionBlurSettings struct {
	Intensity  float32
	MaxSamples uint16
}

// ChromaticAberration configures the chromatic aberration effect (HDR).
type ChromaticAberration struct {
	Intensity   float32
	RadialPower float32
}

// FilmGrain configures the film grain effect (HDR).
type FilmGrain struct {
	Intensity float32
	GrainSize float32
	Animated  bool
}

// FxaaSettings configures FXAA anti-aliasing (SlotSpatialAA, LDR).
// Mutually exclusive with SmaaSettings on the same camera.
type FxaaSettings struct{ Quality uint8 }

// SmaaSettings configures SMAA anti-aliasing (SlotSpatialAA, LDR).
// Mutually exclusive with FxaaSettings; preferred when both are present.
type SmaaSettings struct{ Quality uint8 }

// SsrSettings configures screen-space reflections (SlotSSR, HDR).
type SsrSettings struct {
	MaxSteps   uint16
	MaxBounces uint8
	Thickness  float32
}
