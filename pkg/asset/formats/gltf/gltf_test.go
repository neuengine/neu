package gltf

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"io"
	"testing"

	"github.com/neuengine/neu/pkg/asset"
	renderimage "github.com/neuengine/neu/pkg/render/image"
	"github.com/neuengine/neu/pkg/render/mesh"
)

// ── test-data builders ──────────────────────────────────────────────────────

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

// testBuffer builds the binary backing buffer for the two-mesh test asset:
// a triangle (3 verts) and a quad (4 verts), positions + indices.
func testBuffer() []byte {
	tri := encodeFloats(0, 0, 0, 1, 0, 0, 0, 1, 0)          // 36 B  @ 0
	triIdx := encodeU16(0, 1, 2)                            // 6 B   @ 36
	quad := encodeFloats(0, 0, 0, 1, 0, 0, 1, 1, 0, 0, 1, 0) // 48 B  @ 42
	quadIdx := encodeU16(0, 1, 2, 0, 2, 3)                  // 12 B  @ 90
	out := make([]byte, 0, 102)
	out = append(out, tri...)
	out = append(out, triIdx...)
	out = append(out, quad...)
	out = append(out, quadIdx...)
	return out
}

// gltfJSON returns the glTF JSON for the two-mesh asset. When bufferLine is
// non-empty it is used verbatim as the single buffer entry (data-URI form);
// otherwise a uri-less GLB buffer of byteLength bytes is declared.
func gltfJSON(bufferLine string) string {
	return `{
  "asset": {"version": "2.0", "generator": "neu-test"},
  "scene": 0,
  "scenes": [{"name": "root", "nodes": [0, 1]}],
  "nodes": [{"name": "tri", "mesh": 0}, {"name": "quad", "mesh": 1}],
  "meshes": [
    {"name": "triangle", "primitives": [{"attributes": {"POSITION": 0}, "indices": 1, "material": 0}]},
    {"name": "quad",     "primitives": [{"attributes": {"POSITION": 2}, "indices": 3, "material": 0}]}
  ],
  "materials": [
    {"name": "red", "pbrMetallicRoughness": {"baseColorFactor": [1, 0, 0, 1], "metallicFactor": 0.25, "roughnessFactor": 0.75},
     "emissiveFactor": [0, 0, 0], "alphaMode": "OPAQUE", "doubleSided": true}
  ],
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
  "buffers": [` + bufferLine + `]
}`
}

func embeddedGltf() []byte {
	bin := testBuffer()
	uri := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin)
	line := fmt.Sprintf(`{"uri": %q, "byteLength": %d}`, uri, len(bin))
	return []byte(gltfJSON(line))
}

// makeGLB wraps a JSON chunk and optional BIN chunk into a GLB v2 container.
func makeGLB(jsonChunk, bin []byte) []byte {
	pad := func(b []byte, padByte byte) []byte {
		for len(b)%4 != 0 {
			b = append(b, padByte)
		}
		return b
	}
	jsonChunk = pad(jsonChunk, ' ')
	total := 12 + 8 + len(jsonChunk)
	if bin != nil {
		bin = pad(bin, 0)
		total += 8 + len(bin)
	}
	var out bytes.Buffer
	w := func(v uint32) { _ = binary.Write(&out, binary.LittleEndian, v) }
	w(glbMagic)
	w(2)
	w(uint32(total))
	w(uint32(len(jsonChunk)))
	w(glbChunkJSON)
	out.Write(jsonChunk)
	if bin != nil {
		w(uint32(len(bin)))
		w(glbChunkBIN)
		out.Write(bin)
	}
	return out.Bytes()
}

func binaryGltf() []byte {
	bin := testBuffer()
	jsonChunk := []byte(gltfJSON(fmt.Sprintf(`{"byteLength": %d}`, len(bin))))
	return makeGLB(jsonChunk, bin)
}

// ── tests ───────────────────────────────────────────────────────────────────

func assertTwoMeshAsset(t *testing.T, a GltfAsset) {
	t.Helper()
	if len(a.Meshes) != 2 {
		t.Fatalf("Meshes = %d, want 2", len(a.Meshes))
	}
	if got := a.Meshes[0].VertexCount(); got != 3 {
		t.Errorf("Mesh(0) vertexCount = %d, want 3", got)
	}
	if got := a.Meshes[1].VertexCount(); got != 4 {
		t.Errorf("Mesh(1) vertexCount = %d, want 4", got)
	}
	if len(a.Materials) != 1 {
		t.Fatalf("Materials = %d, want 1", len(a.Materials))
	}
	if a.MeshMaterial(0) != 0 || a.MeshMaterial(1) != 0 {
		t.Errorf("MeshMaterial = (%d,%d), want (0,0)", a.MeshMaterial(0), a.MeshMaterial(1))
	}
	if a.MeshMaterial(99) != -1 {
		t.Error("MeshMaterial(out-of-range) should be -1")
	}
	m := a.Materials[0]
	if got := m.Params.Floats["metallic"]; got != 0.25 {
		t.Errorf("metallic = %v, want 0.25", got)
	}
	if got := m.Params.Floats["roughness"]; got != 0.75 {
		t.Errorf("roughness = %v, want 0.75", got)
	}
	if c := m.Params.Colors["base_color"]; c.R != 1 || c.G != 0 || c.B != 0 || c.A != 1 {
		t.Errorf("base_color = %+v, want red", c)
	}
	if !m.DoubleSided {
		t.Error("DoubleSided should be true")
	}
	if len(a.Scenes) != 1 || a.Scenes[0].Name != "root" || len(a.Scenes[0].Nodes) != 2 {
		t.Errorf("scene fan-out wrong: %+v", a.Scenes)
	}
	if a.DefaultScene != 0 {
		t.Errorf("DefaultScene = %d, want 0", a.DefaultScene)
	}
}

func TestDecodeEmbeddedGltf(t *testing.T) {
	t.Parallel()
	a, err := Decode(embeddedGltf())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	assertTwoMeshAsset(t, a)
}

func TestDecodeBinaryGLB(t *testing.T) {
	t.Parallel()
	a, err := Decode(binaryGltf())
	if err != nil {
		t.Fatalf("Decode GLB: %v", err)
	}
	assertTwoMeshAsset(t, a)
}

func TestLoaderLoadAndExtensions(t *testing.T) {
	t.Parallel()
	l := GltfLoader{}
	exts := map[string]bool{}
	for _, e := range l.Extensions() {
		exts[e] = true
	}
	if !exts[".gltf"] || !exts[".glb"] {
		t.Errorf("Extensions = %v, want .gltf + .glb", l.Extensions())
	}
	a, err := l.Load(bytes.NewReader(embeddedGltf()), LoadSettings{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	assertTwoMeshAsset(t, a)
}

// fanOutHash hashes the fan-out structure: every label plus the salient data
// behind it. Identical hashes across reloads prove label/order stability (INV-4).
func fanOutHash(a GltfAsset) uint64 {
	h := fnv.New64a()
	for _, l := range a.Labels() {
		_, _ = io.WriteString(h, l.String())
		v, ok := a.Get(l)
		if !ok {
			_, _ = io.WriteString(h, "MISS")
			continue
		}
		switch t := v.(type) {
		case *mesh.Mesh:
			_ = binary.Write(h, binary.LittleEndian, int64(t.VertexCount()))
		case GltfScene:
			_, _ = io.WriteString(h, t.Name)
		}
	}
	return h.Sum64()
}

func TestLabelStabilityAcrossReloads(t *testing.T) {
	t.Parallel()
	raw := embeddedGltf()
	first, err := Decode(raw)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	want := fanOutHash(first)

	// INV-4: Mesh(0) always resolves to the 3-vertex triangle.
	v, ok := first.Get(GltfAssetLabel{Kind: KindMesh, Index: 0})
	if !ok || v.(*mesh.Mesh).VertexCount() != 3 {
		t.Fatalf("Mesh(0) did not resolve to the triangle: %v ok=%v", v, ok)
	}

	for i := range 20 {
		got, err := Decode(raw)
		if err != nil {
			t.Fatalf("reload %d: %v", i, err)
		}
		if h := fanOutHash(got); h != want {
			t.Fatalf("reload %d: fan-out hash %d != %d (INV-4 unstable)", i, h, want)
		}
	}
}

func TestLabelsAndStringForms(t *testing.T) {
	t.Parallel()
	a, err := Decode(embeddedGltf())
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	labels := a.Labels()
	// Sorted by (kind, index): Scene(0) < Mesh(0) < Mesh(1) < Material(0).
	want := []string{"Scene(0)", "Mesh(0)", "Mesh(1)", "Material(0)"}
	if len(labels) != len(want) {
		t.Fatalf("Labels = %v, want %v", labels, want)
	}
	for i, l := range labels {
		if l.String() != want[i] {
			t.Errorf("label[%d] = %q, want %q", i, l.String(), want[i])
		}
	}
	// Get on an absent label fails; ErrSubAssetMissing renders the label.
	if _, ok := a.Get(GltfAssetLabel{Kind: KindAnimation, Index: 5}); ok {
		t.Error("Get(Animation(5)) should miss")
	}
	missing := ErrSubAssetMissing{Label: GltfAssetLabel{Kind: KindSkin, Index: 2}}
	if got := missing.Error(); got != "gltf: sub-asset Skin(2) not present" {
		t.Errorf("ErrSubAssetMissing = %q", got)
	}
}

func TestGltfKindStrings(t *testing.T) {
	t.Parallel()
	cases := map[GltfKind]string{
		KindScene: "Scene", KindMesh: "Mesh", KindMaterial: "Material",
		KindTexture: "Texture", KindAnimation: "Animation", KindSkin: "Skin",
		KindMorphTarget: "MorphTarget", GltfKind(200): "Unknown",
	}
	for k, want := range cases {
		if got := k.String(); got != want {
			t.Errorf("GltfKind(%d) = %q, want %q", k, got, want)
		}
	}
}

func TestEmbeddedTexture(t *testing.T) {
	t.Parallel()
	// 2×2 PNG embedded as a data-URI image referenced by a texture.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBuf.Bytes())
	doc := fmt.Sprintf(`{
      "asset": {"version": "2.0"},
      "images":   [{"uri": %q, "mimeType": "image/png"}],
      "textures": [{"name": "albedo", "source": 0}]
    }`, dataURI)

	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(a.Textures) != 1 {
		t.Fatalf("Textures = %d, want 1", len(a.Textures))
	}
	tex := a.Textures[0]
	if tex.Width != 2 || tex.Height != 2 {
		t.Errorf("texture = %dx%d, want 2x2", tex.Width, tex.Height)
	}
	got, ok := a.Get(GltfAssetLabel{Kind: KindTexture, Index: 0})
	if !ok {
		t.Fatal("Texture(0) should resolve")
	}
	if img, isImg := got.(renderimage.Image); !isImg || img.Width != 2 {
		t.Errorf("Texture(0) = %T (%v), want renderimage.Image 2px wide", got, ok)
	}
}

func TestRegisterAll(t *testing.T) {
	t.Parallel()
	if err := RegisterAll(nil); err == nil {
		t.Error("RegisterAll(nil) should error")
	}
	srv := asset.NewAssetServer(asset.NewVFS(), nil)
	if err := RegisterAll(srv); err != nil {
		t.Errorf("RegisterAll(server) = %v", err)
	}
}

// ── error-path tests (INV-2: full asset or error, never partial) ─────────────

func TestDecodeErrors(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		doc  string
	}{
		{"invalid json", `{ this is not json `},
		{"missing position", `{"asset":{"version":"2.0"},"meshes":[{"primitives":[{"attributes":{}}]}]}`},
		{"accessor out of range", `{"asset":{"version":"2.0"},
			"meshes":[{"primitives":[{"attributes":{"POSITION":9}}]}]}`},
		{"external buffer uri", `{"asset":{"version":"2.0"},
			"buffers":[{"uri":"model.bin","byteLength":4}],
			"bufferViews":[{"buffer":0,"byteLength":4}],
			"accessors":[{"bufferView":0,"componentType":5126,"count":1,"type":"SCALAR"}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}]}`},
		{"bad base64", `{"asset":{"version":"2.0"},
			"buffers":[{"uri":"data:application/octet-stream;base64,@@@@","byteLength":4}]}`},
		{"non-base64 data uri", `{"asset":{"version":"2.0"},
			"buffers":[{"uri":"data:text/plain,hello","byteLength":5}]}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Decode([]byte(tc.doc)); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

func TestExternalBufferIsTypedError(t *testing.T) {
	t.Parallel()
	doc := `{"asset":{"version":"2.0"},"buffers":[{"uri":"x.bin","byteLength":4}]}`
	_, err := Decode([]byte(doc))
	if !errors.Is(err, ErrExternalResource) {
		t.Errorf("err = %v, want ErrExternalResource", err)
	}
}

func TestGLBErrors(t *testing.T) {
	t.Parallel()
	// Truncated header.
	if _, _, err := splitContainer([]byte("glTF\x02")); !errors.Is(err, ErrInvalidGLB) {
		t.Errorf("truncated GLB err = %v", err)
	}
	// Bad version.
	bad := make([]byte, 12)
	binary.LittleEndian.PutUint32(bad[0:], glbMagic)
	binary.LittleEndian.PutUint32(bad[4:], 1) // version 1
	binary.LittleEndian.PutUint32(bad[8:], 12)
	if _, _, err := splitContainer(bad); !errors.Is(err, ErrInvalidGLB) {
		t.Errorf("bad-version GLB err = %v", err)
	}
	// Declared length mismatch.
	binary.LittleEndian.PutUint32(bad[4:], 2)
	binary.LittleEndian.PutUint32(bad[8:], 999)
	if _, _, err := splitContainer(bad); !errors.Is(err, ErrInvalidGLB) {
		t.Errorf("length-mismatch GLB err = %v", err)
	}
	// Missing JSON chunk (valid header, no chunks).
	binary.LittleEndian.PutUint32(bad[8:], 12)
	if _, _, err := splitContainer(bad); !errors.Is(err, ErrInvalidGLB) {
		t.Errorf("missing-JSON GLB err = %v", err)
	}
}

func TestIndexWideningAndUByte(t *testing.T) {
	t.Parallel()
	// POSITION (3 floats) + UNSIGNED_BYTE indices [0,1,2] → widened to u16.
	pos := encodeFloats(0, 0, 0, 1, 0, 0, 0, 1, 0)
	idx := []byte{0, 1, 2}
	bin := append(append([]byte{}, pos...), idx...)
	uri := "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin)
	doc := fmt.Sprintf(`{"asset":{"version":"2.0"},
		"buffers":[{"uri":%q,"byteLength":%d}],
		"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":3}],
		"accessors":[
			{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":1,"componentType":5121,"count":3,"type":"SCALAR"}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1,"mode":4}]}]}`, uri, len(bin))

	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ib := a.Meshes[0].Indices()
	if ib == nil || ib.Wide || len(ib.Data) != 6 {
		t.Fatalf("widened index buffer wrong: %+v", ib)
	}
}

func TestTopologyMapping(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode uint32
		want mesh.PrimitiveTopology
	}{
		{0, mesh.TopologyPointList},
		{1, mesh.TopologyLineList},
		{3, mesh.TopologyLineStrip},
		{4, mesh.TopologyTriangleList},
		{5, mesh.TopologyTriangleStrip},
		{6, mesh.TopologyTriangleList}, // fan → list fallback
	}
	for _, tc := range cases {
		m := tc.mode
		if got := topology(&m); got != tc.want {
			t.Errorf("topology(%d) = %v, want %v", tc.mode, got, tc.want)
		}
	}
	if got := topology(nil); got != mesh.TopologyTriangleList {
		t.Errorf("topology(nil) = %v, want TriangleList", got)
	}
}

// failingReader always errors — exercises the Loader.Load read-error path.
type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("boom") }

func TestLoaderReadError(t *testing.T) {
	t.Parallel()
	if _, err := (GltfLoader{}).Load(failingReader{}, LoadSettings{}); err == nil {
		t.Error("Load should propagate a read error")
	}
}

// FuzzDecode asserts the loader never panics on arbitrary input (INV-2 / L1 §2).
func FuzzDecode(f *testing.F) {
	f.Add(embeddedGltf())
	f.Add(binaryGltf())
	f.Add([]byte("glTF\x02\x00\x00\x00"))
	f.Add([]byte(`{"asset":{"version":"2.0"}}`))
	f.Add([]byte("not gltf at all"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = Decode(data) // must return, never panic
	})
}

func dataURI(bin []byte) string {
	return "data:application/octet-stream;base64," + base64.StdEncoding.EncodeToString(bin)
}

// TestFullAttributesAndWideIndices exercises VEC2/VEC4 accessor types, multiple
// vertex attributes, and UNSIGNED_INT (wide) indices.
func TestFullAttributesAndWideIndices(t *testing.T) {
	t.Parallel()
	pos := encodeFloats(0, 0, 0, 1, 0, 0, 0, 1, 0)          // VEC3 @ 0   (36)
	nrm := encodeFloats(0, 0, 1, 0, 0, 1, 0, 0, 1)          // VEC3 @ 36  (36)
	uv := encodeFloats(0, 0, 1, 0, 0, 1)                    // VEC2 @ 72  (24)
	col := encodeFloats(1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1) // VEC4 @ 96  (48)
	idx := make([]byte, 12)                                 // UINT @ 144 (12)
	binary.LittleEndian.PutUint32(idx[0:], 0)
	binary.LittleEndian.PutUint32(idx[4:], 1)
	binary.LittleEndian.PutUint32(idx[8:], 2)
	bin := bytes.Join([][]byte{pos, nrm, uv, col, idx}, nil)

	doc := fmt.Sprintf(`{"asset":{"version":"2.0"},
		"buffers":[{"uri":%q,"byteLength":%d}],
		"bufferViews":[
			{"buffer":0,"byteOffset":0,"byteLength":36},
			{"buffer":0,"byteOffset":36,"byteLength":36},
			{"buffer":0,"byteOffset":72,"byteLength":24},
			{"buffer":0,"byteOffset":96,"byteLength":48},
			{"buffer":0,"byteOffset":144,"byteLength":12}],
		"accessors":[
			{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":1,"componentType":5126,"count":3,"type":"VEC3"},
			{"bufferView":2,"componentType":5126,"count":3,"type":"VEC2"},
			{"bufferView":3,"componentType":5126,"count":3,"type":"VEC4"},
			{"bufferView":4,"componentType":5125,"count":3,"type":"SCALAR"}],
		"meshes":[{"primitives":[{"attributes":{"POSITION":0,"NORMAL":1,"TEXCOORD_0":2,"COLOR_0":3},"indices":4}]}]}`,
		dataURI(bin), len(bin))

	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	m := a.Meshes[0]
	attrs := m.Attributes()
	for _, k := range []mesh.AttrKind{mesh.AttrPosition, mesh.AttrNormal, mesh.AttrUV0, mesh.AttrColor} {
		if _, ok := attrs[k]; !ok {
			t.Errorf("missing attribute %v", k)
		}
	}
	if ib := m.Indices(); ib == nil || !ib.Wide {
		t.Errorf("expected wide (uint32) index buffer, got %+v", ib)
	}
}

func TestAttributeAndIndexErrors(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, doc string }{
		// POSITION declared as USHORT (non-float) → unsupported component error.
		{"non-float position", `{"asset":{"version":"2.0"},
			"buffers":[{"uri":"data:application/octet-stream;base64,AAAAAAAAAAA=","byteLength":8}],
			"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":8}],
			"accessors":[{"bufferView":0,"componentType":5123,"count":2,"type":"VEC4"}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}]}`},
		// Index accessor of VEC2 (non-scalar).
		{"non-scalar indices", fmt.Sprintf(`{"asset":{"version":"2.0"},
			"buffers":[{"uri":%q,"byteLength":48}],
			"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":12}],
			"accessors":[
				{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
				{"bufferView":1,"componentType":5125,"count":1,"type":"VEC3"}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1}]}]}`,
			dataURI(make([]byte, 48)))},
		// Float index componentType (unsupported).
		{"float indices", fmt.Sprintf(`{"asset":{"version":"2.0"},
			"buffers":[{"uri":%q,"byteLength":48}],
			"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36},{"buffer":0,"byteOffset":36,"byteLength":12}],
			"accessors":[
				{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"},
				{"bufferView":1,"componentType":5126,"count":3,"type":"SCALAR"}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0},"indices":1}]}]}`,
			dataURI(make([]byte, 48)))},
		// Unknown accessor type.
		{"unknown accessor type", fmt.Sprintf(`{"asset":{"version":"2.0"},
			"buffers":[{"uri":%q,"byteLength":36}],
			"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":36}],
			"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"WAT"}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}]}`, dataURI(make([]byte, 36)))},
		// Accessor reads past the buffer end.
		{"accessor overruns buffer", fmt.Sprintf(`{"asset":{"version":"2.0"},
			"buffers":[{"uri":%q,"byteLength":12}],
			"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":12}],
			"accessors":[{"bufferView":0,"componentType":5126,"count":3,"type":"VEC3"}],
			"meshes":[{"primitives":[{"attributes":{"POSITION":0}}]}]}`, dataURI(make([]byte, 12)))},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := Decode([]byte(tc.doc)); err == nil {
				t.Errorf("%s: expected error, got nil", tc.name)
			}
		})
	}
}

func TestTextureViaBufferViewAndErrors(t *testing.T) {
	t.Parallel()
	// Embed a 2×2 PNG inside the buffer and reference it through a bufferView.
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(1, 1, color.RGBA{B: 255, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatal(err)
	}
	pngBytes := pngBuf.Bytes()
	doc := fmt.Sprintf(`{"asset":{"version":"2.0"},
		"buffers":[{"uri":%q,"byteLength":%d}],
		"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":%d}],
		"images":[{"bufferView":0,"mimeType":"image/png"}],
		"textures":[{"source":0}]}`, dataURI(pngBytes), len(pngBytes), len(pngBytes))

	a, err := Decode([]byte(doc))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(a.Textures) != 1 || a.Textures[0].Width != 2 {
		t.Fatalf("bufferView texture wrong: %+v", a.Textures)
	}

	// Texture source out of range.
	bad := `{"asset":{"version":"2.0"},"images":[],"textures":[{"source":3}]}`
	if _, err := Decode([]byte(bad)); err == nil {
		t.Error("texture with out-of-range source should error")
	}
	// External-URI image is unsupported standalone.
	ext := `{"asset":{"version":"2.0"},"images":[{"uri":"albedo.png"}],"textures":[{"source":0}]}`
	if _, err := Decode([]byte(ext)); !errors.Is(err, ErrExternalResource) {
		t.Errorf("external image err = %v, want ErrExternalResource", err)
	}
	// Garbage image bytes fail to decode.
	garbage := fmt.Sprintf(`{"asset":{"version":"2.0"},
		"buffers":[{"uri":%q,"byteLength":4}],
		"bufferViews":[{"buffer":0,"byteOffset":0,"byteLength":4}],
		"images":[{"bufferView":0,"mimeType":"image/png"}],
		"textures":[{"source":0}]}`, dataURI([]byte{1, 2, 3, 4}))
	if _, err := Decode([]byte(garbage)); err == nil {
		t.Error("garbage texture bytes should fail to decode")
	}
}
