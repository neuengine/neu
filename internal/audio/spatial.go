package audio

import "math"

// Attenuation computes the volume multiplier based on distance from a source.
// model selects the attenuation formula; dist is the source-listener distance;
// refDist is the reference distance (full volume); maxDist is the cutoff.
func Attenuation(model uint8, dist, refDist, maxDist float32) float32 {
	if dist <= refDist {
		return 1
	}
	if maxDist > 0 && dist >= maxDist {
		return 0
	}
	switch model {
	case 1: // Linear
		return 1 - (dist-refDist)/(maxDist-refDist)
	default: // InverseDistance
		if dist == 0 {
			return 1
		}
		return refDist / dist
	}
}

// StereoPan computes left and right gain coefficients from listener-relative
// angle. listenerForward and sourceDir are unit vectors in listener space.
// Returns (left, right) in [0, 1].
func StereoPan(listenerRight, sourceDir [3]float32) (left, right float32) {
	// Project source direction onto listener right vector.
	dot := listenerRight[0]*sourceDir[0] +
		listenerRight[1]*sourceDir[1] +
		listenerRight[2]*sourceDir[2]
	// dot ∈ [-1, 1]: -1 = full left, +1 = full right
	angle := float32(math.Asin(float64(clamp1(dot))))
	// Constant-power panning
	right = float32(math.Cos(float64(angle*math.Pi/4 + math.Pi/4)))
	left = float32(math.Sin(float64(angle*math.Pi/4 + math.Pi/4)))
	return
}

func clamp1(v float32) float32 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}
