package postprocess

import (
	"github.com/neuengine/neu/pkg/asset"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// ColorGrading configures the color grading pass (SlotColorGrading, HDR).
// All scalar fields are multiplicative/additive adjustments around their
// identity values (Exposure=0, Gamma=1, Saturation=1, Contrast=1).
type ColorGrading struct {
	// Exposure is an additive EV adjustment applied before grading.
	Exposure float32
	// Gamma applies a power-law correction: output = linear^(1/Gamma).
	// Identity = 1.0.
	Gamma float32
	// Saturation scales colour saturation. 0 = greyscale, 1 = identity.
	Saturation float32
	// Contrast scales luminance around mid-grey. 1 = identity.
	Contrast float32
	// LUT is an optional 3-D look-up table asset (identity if unloaded).
	LUT asset.Handle[renderimage.Image]
}
