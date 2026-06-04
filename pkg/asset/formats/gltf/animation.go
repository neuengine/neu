package gltf

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/neuengine/neu/pkg/animation"
)

// buildAnimations converts every glTF animation into an animation.AnimationClip.
// Each channel becomes a VariableCurve: the sampler's input accessor supplies the
// keyframe times and the output accessor the values, the target node's name is
// the EntityPath, and the glTF path maps to a Transform reflection property.
// A clip's Duration is the largest keyframe time across its curves. Unsupported
// target paths (e.g. "weights"/morph) are skipped rather than failing the load.
func (d *decoder) buildAnimations() ([]animation.AnimationClip, error) {
	if len(d.doc.Animations) == 0 {
		return nil, nil
	}
	clips := make([]animation.AnimationClip, 0, len(d.doc.Animations))
	for ai, ga := range d.doc.Animations {
		var clip animation.AnimationClip
		for ci, ch := range ga.Channels {
			prop, ok := pathProperty(ch.Target.Path)
			if !ok {
				continue
			}
			if int(ch.Sampler) >= len(ga.Samplers) {
				return nil, fmt.Errorf("gltf: animation %d channel %d: sampler %d out of range", ai, ci, ch.Sampler)
			}
			samp := ga.Samplers[ch.Sampler]

			times, tcc, err := d.readFloatAccessor(samp.Input)
			if err != nil {
				return nil, fmt.Errorf("gltf: animation %d channel %d input: %w", ai, ci, err)
			}
			if tcc != 1 {
				return nil, fmt.Errorf("gltf: animation %d channel %d: input must be SCALAR", ai, ci)
			}
			values, vcc, err := d.readFloatAccessor(samp.Output)
			if err != nil {
				return nil, fmt.Errorf("gltf: animation %d channel %d output: %w", ai, ci, err)
			}

			kf := animation.Keyframes{Times: times, Interp: interpolation(samp.Interpolation)}
			if kf.Interp == animation.InterpolationCubicSpline {
				kf.Values, kf.Tangents, err = splitCubicSpline(values, len(times), vcc)
				if err != nil {
					return nil, fmt.Errorf("gltf: animation %d channel %d: %w", ai, ci, err)
				}
			} else {
				kf.Values = values
			}

			clip.Curves = append(clip.Curves, animation.VariableCurve{
				Target:    animation.AnimationTargetId{EntityPath: d.nodeName(ch.Target.Node), Property: prop},
				Keyframes: kf,
			})
			if len(times) > 0 {
				if last := times[len(times)-1]; last > clip.Duration {
					clip.Duration = last
				}
			}
		}
		clips = append(clips, clip)
	}
	return clips, nil
}

// splitCubicSpline separates a glTF CUBICSPLINE output stream — stored as
// [inTangent, value, outTangent] triplets per keyframe — into the engine's
// (values, tangents) layout: values is one value-vector per keyframe; tangents is
// the in/out-tangent pair per keyframe (len = 2 * keyframes * stride).
func splitCubicSpline(out []float32, keyframes, stride int) (values, tangents []float32, err error) {
	if stride <= 0 || keyframes <= 0 {
		return nil, nil, nil
	}
	if len(out) != keyframes*3*stride {
		return nil, nil, fmt.Errorf("cubicspline output has %d floats, want %d (3 triplets × %d keyframes × %d stride)",
			len(out), keyframes*3*stride, keyframes, stride)
	}
	values = make([]float32, 0, keyframes*stride)
	tangents = make([]float32, 0, 2*keyframes*stride)
	for k := range keyframes {
		base := k * 3 * stride
		inTan := out[base : base+stride]
		value := out[base+stride : base+2*stride]
		outTan := out[base+2*stride : base+3*stride]
		values = append(values, value...)
		tangents = append(tangents, inTan...)
		tangents = append(tangents, outTan...)
	}
	return values, tangents, nil
}

// readFloatAccessor reads a FLOAT accessor into a flat []float32, returning the
// values plus the per-element component count (1 for SCALAR, 3 for VEC3, …).
func (d *decoder) readFloatAccessor(i uint32) (vals []float32, compCount int, err error) {
	data, compType, cc, count, err := d.accessorData(i)
	if err != nil {
		return nil, 0, err
	}
	if compType != compFloat {
		return nil, 0, fmt.Errorf("gltf: accessor %d: expected FLOAT components, got %d", i, compType)
	}
	return bytesToFloats(data, int(count)*cc), cc, nil
}

// bytesToFloats decodes n little-endian float32s from a tightly packed buffer.
func bytesToFloats(data []byte, n int) []float32 {
	out := make([]float32, n)
	for i := range n {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4 : i*4+4]))
	}
	return out
}

// nodeName returns the target node's name, or "" for a node-less target (which
// animates the player entity itself).
func (d *decoder) nodeName(node *uint32) string {
	if node == nil || int(*node) >= len(d.doc.Nodes) {
		return ""
	}
	return d.doc.Nodes[*node].Name
}

// pathProperty maps a glTF animation target path to a Transform reflection path.
// "weights" (morph targets) is not yet mapped and reports ok=false.
func pathProperty(path string) (string, bool) {
	switch path {
	case "translation":
		return "Transform.Translation", true
	case "rotation":
		return "Transform.Rotation", true
	case "scale":
		return "Transform.Scale", true
	}
	return "", false
}

// interpolation maps a glTF sampler interpolation mode to the engine's; an
// absent or unknown value defaults to LINEAR (the glTF default).
func interpolation(s string) animation.Interpolation {
	switch s {
	case "STEP":
		return animation.InterpolationStep
	case "CUBICSPLINE":
		return animation.InterpolationCubicSpline
	}
	return animation.InterpolationLinear
}
