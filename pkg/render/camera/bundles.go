package camera

import pkgmath "github.com/neuengine/neu/pkg/math"

// Camera3D returns a slice of component values for a standard 3-D perspective
// camera. Attach to an entity via World.Spawn or World.Insert.
//
// Default: perspective 60° FoV, near=0.1, far=1000, window-target, active.
func Camera3D() []any {
	return []any{
		Camera{
			Target: RenderTarget{Kind: TargetWindow},
			Clear: ClearColorConfig{
				Mode:  ClearCustom,
				Color: pkgmath.LinearRgba{R: 0.1, G: 0.1, B: 0.1, A: 1},
			},
			HDR:      false,
			Order:    0,
			IsActive: true,
		},
		PerspectiveProjection{
			FovY:   1.0472, // 60° in radians
			Aspect: 1.7778, // 16:9 default; updated by CameraUpdateSystems
			Near:   0.1,
			Far:    1000,
		},
	}
}

// Camera2D returns a slice of component values for a standard 2-D orthographic
// camera. Attach to an entity via World.Spawn or World.Insert.
//
// Default: 1:1 pixel mapping, near=-1000, far=1000, window-target, active.
func Camera2D() []any {
	return []any{
		Camera{
			Target: RenderTarget{Kind: TargetWindow},
			Clear: ClearColorConfig{
				Mode:  ClearCustom,
				Color: pkgmath.LinearRgba{R: 0.1, G: 0.1, B: 0.1, A: 1},
			},
			HDR:      false,
			Order:    0,
			IsActive: true,
		},
		OrthographicProjection{
			Left: -1, Right: 1, Bottom: -1, Top: 1,
			Near:    -1000,
			Far:     1000,
			Scaling: ScalingWindowSize,
		},
	}
}
