package postprocess

import (
	"github.com/neuengine/neu/pkg/asset"
	renderimage "github.com/neuengine/neu/pkg/render/image"
)

// ColorGrading configures the color grading pass (SlotColorGrading, HDR).
// All scalar fields are multiplicative/additive adjustments around their
// identity values (Exposure=0, Gamma=1, Saturation=1, Contrast=1).
type ColorGrading struct {
	LUT        asset.Handle[renderimage.Image]
	Exposure   float32
	Gamma      float32
	Saturation float32
	Contrast   float32
}
