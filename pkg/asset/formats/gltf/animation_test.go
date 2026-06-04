package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/neuengine/neu/pkg/animation"
	"github.com/neuengine/neu/pkg/render/mesh"
)

// animDataURI packs the given float32s little-endian into a base64 data URI.
func animDataURI(floats ...float32) string {
	var buf bytes.Buffer
	for _, f := range floats {
		_ = binary.Write(&buf, binary.LittleEndian, f)
	}
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestDecodeLinearTranslationAnimation(t *testing.T) {
	t.Parallel()
	// Buffer: times [0,1] (8 bytes) then translations [(0,0,0),(1,2,3)] (24 bytes).
	uri := animDataURI(0, 1, 0, 0, 0, 1, 2, 3)
	doc := fmt.Sprintf(`{
	  "asset":{"version":"2.0"},
	  "nodes":[{"name":"Cube"}],
	  "buffers":[{"uri":%q,"byteLength":32}],
	  "bufferViews":[
	    {"buffer":0,"byteOffset":0,"byteLength":8},
	    {"buffer":0,"byteOffset":8,"byteLength":24}
	  ],
	  "accessors":[
	    {"bufferView":0,"componentType":5126,"count":2,"type":"SCALAR"},
	    {"bufferView":1,"componentType":5126,"count":2,"type":"VEC3"}
	  ],
	  "animations":[{
	    "channels":[{"target":{"node":0,"path":"translation"},"sampler":0}],
	    "samplers":[{"input":0,"output":1,"interpolation":"LINEAR"}]
	  }]
	}`, uri)

	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(a.Animations) != 1 {
		t.Fatalf("got %d animations, want 1", len(a.Animations))
	}
	clip := a.Animations[0]
	if len(clip.Curves) != 1 {
		t.Fatalf("clip has %d curves, want 1", len(clip.Curves))
	}
	c := clip.Curves[0]
	if c.Target.EntityPath != "Cube" || c.Target.Property != "Transform.Translation" {
		t.Errorf("target = %+v, want {Cube, Transform.Translation}", c.Target)
	}
	if !reflect.DeepEqual(c.Keyframes.Times, []float32{0, 1}) {
		t.Errorf("times = %v, want [0 1]", c.Keyframes.Times)
	}
	if !reflect.DeepEqual(c.Keyframes.Values, []float32{0, 0, 0, 1, 2, 3}) {
		t.Errorf("values = %v, want [0 0 0 1 2 3]", c.Keyframes.Values)
	}
	if c.Keyframes.Interp != animation.InterpolationLinear {
		t.Errorf("interp = %v, want Linear", c.Keyframes.Interp)
	}
	if clip.Duration != 1 {
		t.Errorf("duration = %v, want 1", clip.Duration)
	}

	// Addressable via its stable label.
	v, ok := a.Get(GltfAssetLabel{Kind: KindAnimation, Index: 0})
	if got, isClip := v.(animation.AnimationClip); !ok || !isClip || len(got.Curves) != 1 {
		t.Errorf("Get(Animation(0)) = %v (ok=%v), want the clip", v, ok)
	}
}

func TestDecodeAnimationSkipsUnsupportedPath(t *testing.T) {
	t.Parallel()
	uri := animDataURI(0, 1, 0.5, 0.5)
	// A "weights" (morph) channel is not mapped → skipped, leaving an empty clip.
	doc := fmt.Sprintf(`{
	  "asset":{"version":"2.0"},
	  "nodes":[{"name":"Cube"}],
	  "buffers":[{"uri":%q,"byteLength":16}],
	  "bufferViews":[
	    {"buffer":0,"byteOffset":0,"byteLength":8},
	    {"buffer":0,"byteOffset":8,"byteLength":8}
	  ],
	  "accessors":[
	    {"bufferView":0,"componentType":5126,"count":2,"type":"SCALAR"},
	    {"bufferView":1,"componentType":5126,"count":2,"type":"SCALAR"}
	  ],
	  "animations":[{
	    "channels":[{"target":{"node":0,"path":"weights"},"sampler":0}],
	    "samplers":[{"input":0,"output":1}]
	  }]
	}`, uri)
	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(a.Animations) != 1 || len(a.Animations[0].Curves) != 0 {
		t.Errorf("unsupported-path animation should have 0 curves, got %d", len(a.Animations[0].Curves))
	}
}

func TestSplitCubicSpline(t *testing.T) {
	t.Parallel()
	// 2 keyframes, VEC3 (stride 3): [inTan, value, outTan] per keyframe.
	out := []float32{
		11, 11, 11, 1, 1, 1, 99, 99, 99, // kf0
		22, 22, 22, 2, 2, 2, 88, 88, 88, // kf1
	}
	vals, tans, err := splitCubicSpline(out, 2, 3)
	if err != nil {
		t.Fatalf("splitCubicSpline: %v", err)
	}
	if !reflect.DeepEqual(vals, []float32{1, 1, 1, 2, 2, 2}) {
		t.Errorf("values = %v, want [1 1 1 2 2 2]", vals)
	}
	wantTan := []float32{11, 11, 11, 99, 99, 99, 22, 22, 22, 88, 88, 88}
	if !reflect.DeepEqual(tans, wantTan) {
		t.Errorf("tangents = %v, want %v", tans, wantTan)
	}
	// Malformed length is an error, not a panic.
	if _, _, err := splitCubicSpline([]float32{1, 2, 3}, 2, 3); err == nil {
		t.Error("expected an error for mismatched cubicspline length")
	}
}

func TestDecodeSkinnedVertexAttributes(t *testing.T) {
	t.Parallel()
	// One vertex: POSITION (vec3 float), JOINTS_0 (vec4 ubyte), WEIGHTS_0 (vec4 float).
	var buf bytes.Buffer
	for _, f := range []float32{0, 0, 0} { // POSITION
		_ = binary.Write(&buf, binary.LittleEndian, f)
	}
	buf.Write([]byte{1, 2, 3, 0})                 // JOINTS_0 (unsigned byte)
	for _, f := range []float32{0.5, 0.5, 0, 0} { // WEIGHTS_0
		_ = binary.Write(&buf, binary.LittleEndian, f)
	}
	uri := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	doc := fmt.Sprintf(`{
	  "asset":{"version":"2.0"},
	  "meshes":[{"primitives":[{"attributes":{"POSITION":0,"JOINTS_0":1,"WEIGHTS_0":2}}]}],
	  "buffers":[{"uri":%q,"byteLength":32}],
	  "bufferViews":[
	    {"buffer":0,"byteOffset":0,"byteLength":12},
	    {"buffer":0,"byteOffset":12,"byteLength":4},
	    {"buffer":0,"byteOffset":16,"byteLength":16}
	  ],
	  "accessors":[
	    {"bufferView":0,"componentType":5126,"count":1,"type":"VEC3"},
	    {"bufferView":1,"componentType":5121,"count":1,"type":"VEC4"},
	    {"bufferView":2,"componentType":5126,"count":1,"type":"VEC4"}
	  ]
	}`, uri)

	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(a.Meshes) != 1 {
		t.Fatalf("got %d meshes, want 1", len(a.Meshes))
	}
	attrs := a.Meshes[0].Attributes()

	ji, ok := attrs[mesh.AttrJointIndices]
	if !ok || ji.Format != mesh.FormatUint16x4 {
		t.Fatalf("joint indices missing or wrong format (ok=%v, fmt=%v)", ok, ji.Format)
	}
	// ubyte (1,2,3,0) widened to LE uint16: [1,0, 2,0, 3,0, 0,0].
	if !bytes.Equal(ji.Data, []byte{1, 0, 2, 0, 3, 0, 0, 0}) {
		t.Errorf("joint-index data = %v, want widened uint16x4", ji.Data)
	}
	if jw, ok := attrs[mesh.AttrJointWeights]; !ok || jw.Format != mesh.FormatFloat32x4 {
		t.Errorf("joint weights missing or wrong format (ok=%v, fmt=%v)", ok, jw.Format)
	}
}

func TestAnimationHelpers(t *testing.T) {
	t.Parallel()
	for path, want := range map[string]string{
		"translation": "Transform.Translation",
		"rotation":    "Transform.Rotation",
		"scale":       "Transform.Scale",
	} {
		if got, ok := pathProperty(path); !ok || got != want {
			t.Errorf("pathProperty(%q) = %q,%v want %q,true", path, got, ok, want)
		}
	}
	if _, ok := pathProperty("weights"); ok {
		t.Error("pathProperty(weights) should be unsupported")
	}
	if interpolation("STEP") != animation.InterpolationStep ||
		interpolation("CUBICSPLINE") != animation.InterpolationCubicSpline ||
		interpolation("") != animation.InterpolationLinear {
		t.Error("interpolation mapping wrong")
	}
}
