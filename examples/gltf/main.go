// examples/gltf demonstrates the glTF multi-asset fan-out: a single embedded
// glTF file decodes into several meshes + a material + a scene, each addressable
// by a deterministic GltfAssetLabel that is stable across reloads (INV-4). The
// fan-out hash is identical across ≥20 runs (C29 round-trip golden gate).
//
// The example is fully self-contained and dep-free (C-003): the glTF buffer is
// built in-memory and embedded as a base64 data URI, then decoded with the
// stdlib-only loader — no GPU, no filesystem.
//
// Bootstrap: validates l2-asset-formats-go (glTF INV-2/INV-4) against
// l1-asset-formats.
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"hash/fnv"
	"io"

	"github.com/neuengine/neu/pkg/asset/formats/gltf"
	"github.com/neuengine/neu/pkg/render/mesh"
)

func encodeFloats(vals ...float32) []byte {
	var buf bytes.Buffer
	for _, v := range vals {
		_ = binary.Write(&buf, binary.LittleEndian, v)
	}
	return buf.Bytes()
}

func encodeU16(vals ...uint16) []byte {
	buf := make([]byte, len(vals)*2)
	for i, v := range vals {
		binary.LittleEndian.PutUint16(buf[i*2:], v)
	}
	return buf
}

// buildGltf assembles a two-mesh (triangle + quad) glTF with one PBR material,
// embedding the binary buffer as a base64 data URI.
func buildGltf() []byte {
	bin := bytes.Join([][]byte{
		encodeFloats(0, 0, 0, 1, 0, 0, 0, 1, 0),          // tri pos    @ 0  (36)
		encodeU16(0, 1, 2),                               // tri idx    @ 36 (6)
		encodeFloats(0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0), // quad pos   @ 42 (48)
		encodeU16(0, 1, 2, 0, 2, 3),                      // quad idx   @ 90 (12)
	}, nil)
	uri := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin)

	return fmt.Appendf(nil, `{
      "asset": {"version": "2.0"},
      "scene": 0,
      "scenes": [{"name": "root", "nodes": [0, 1]}],
      "nodes": [{"name": "tri", "mesh": 0}, {"name": "quad", "mesh": 1}],
      "meshes": [
        {"name": "triangle", "primitives": [{"attributes": {"POSITION": 0}, "indices": 1, "material": 0}]},
        {"name": "quad",     "primitives": [{"attributes": {"POSITION": 2}, "indices": 3, "material": 0}]}
      ],
      "materials": [{"name": "red", "pbrMetallicRoughness": {
        "baseColorFactor": [1, 0, 0, 1], "metallicFactor": 0.25, "roughnessFactor": 0.75}}],
      "accessors": [
        {"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"},
        {"bufferView": 1, "componentType": 5123, "count": 3, "type": "SCALAR"},
        {"bufferView": 2, "componentType": 5126, "count": 4, "type": "VEC3"},
        {"bufferView": 3, "componentType": 5123, "count": 6, "type": "SCALAR"}
      ],
      "bufferViews": [
        {"buffer": 0, "byteOffset": 0,  "byteLength": 36},
        {"buffer": 0, "byteOffset": 36, "byteLength": 6},
        {"buffer": 0, "byteOffset": 42, "byteLength": 48},
        {"buffer": 0, "byteOffset": 90, "byteLength": 12}
      ],
      "buffers": [{"uri": %q, "byteLength": %d}]
    }`, uri, len(bin))
}

// run decodes the embedded glTF, verifies the fan-out, and returns a
// deterministic hash over every labelled sub-asset (stable across reloads, INV-4).
func run() (uint64, error) {
	a, err := gltf.Decode(buildGltf())
	if err != nil {
		return 0, fmt.Errorf("decode: %w", err)
	}

	// Fan-out shape: 2 meshes (one per primitive), 1 material, 1 scene (INV-2).
	if len(a.Meshes) != 2 || len(a.Materials) != 1 || len(a.Scenes) != 1 {
		return 0, fmt.Errorf("fan-out = %d meshes / %d materials / %d scenes, want 2/1/1",
			len(a.Meshes), len(a.Materials), len(a.Scenes))
	}

	// INV-4: Mesh(0) always resolves to the 3-vertex triangle.
	v, ok := a.Get(gltf.GltfAssetLabel{Kind: gltf.KindMesh, Index: 0})
	if !ok {
		return 0, fmt.Errorf("Mesh(0) did not resolve")
	}
	if got := v.(*mesh.Mesh).VertexCount(); got != 3 {
		return 0, fmt.Errorf("Mesh(0) vertexCount = %d, want 3", got)
	}

	// Deterministic hash over the labelled fan-out.
	h := fnv.New64a()
	for _, label := range a.Labels() {
		_, _ = io.WriteString(h, label.String())
		sub, _ := a.Get(label)
		switch t := sub.(type) {
		case *mesh.Mesh:
			_ = binary.Write(h, binary.LittleEndian, int64(t.VertexCount()))
		case gltf.GltfScene:
			_, _ = io.WriteString(h, t.Name)
		}
	}
	return h.Sum64(), nil
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: glTF fan-out hash=%d\n", h)
}
