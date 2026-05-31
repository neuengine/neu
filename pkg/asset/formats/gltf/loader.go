package gltf

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/neuengine/neu/pkg/asset"
	renderimage "github.com/neuengine/neu/pkg/render/image"
	"github.com/neuengine/neu/pkg/render/material"
	"github.com/neuengine/neu/pkg/render/mesh"
)

// Sentinel errors. ErrExternalResource and ErrUnsupportedDataURI mark inputs
// that need App-level resolution (VFS) or an encoding the standalone loader does
// not handle; ErrInvalidGLB marks a malformed binary container.
var (
	ErrExternalResource   = errors.New("gltf: external resource references require the AssetServer/VFS (not supported standalone)")
	ErrUnsupportedDataURI = errors.New("gltf: unsupported data URI encoding")
	ErrInvalidGLB         = errors.New("gltf: invalid GLB container")
)

// ErrSubAssetMissing is returned by [GltfAsset.Get] when a label is absent.
type ErrSubAssetMissing struct{ Label GltfAssetLabel }

func (e ErrSubAssetMissing) Error() string {
	return fmt.Sprintf("gltf: sub-asset %s not present", e.Label)
}

// AxisConvention selects the coordinate-system handedness applied on import.
type AxisConvention uint8

const (
	AxisGltfRightHanded AxisConvention = iota // glTF native: +Y up, right-handed
)

// LoadSettings carries optional glTF decoding hints. The zero value decodes a
// file in its native glTF axis convention with extension-based detection.
type LoadSettings struct {
	Format   string // explicit override; "" = extension auto-detect
	GltfAxis AxisConvention
}

// GltfScene is a lightweight scene record (its node roots). The heavyweight
// scene.DynamicScene conversion is deferred to App integration.
type GltfScene struct {
	Name  string
	Nodes []int
}

// GltfNode is one node of the glTF hierarchy. Mesh is -1 when the node carries
// no geometry. Local transforms are deferred to the scene conversion.
type GltfNode struct {
	Name     string
	Children []int
	Mesh     int
}

// GltfAsset is the multi-asset fan-out of a single glTF file (L1 §4.1).
// Sub-asset slices preserve glTF declaration order, so [GltfAssetLabel] addresses
// are identical across reloads (INV-4). It is itself a single loadable asset; the
// per-sub-asset handle registration is an App-integration concern.
type GltfAsset struct {
	labels       map[GltfAssetLabel]int // (kind,index) → slice index (INV-4)
	Scenes       []GltfScene
	Nodes        []GltfNode
	Meshes       []*mesh.Mesh
	Materials    []*material.Material
	Textures     []renderimage.Image
	meshMaterial []int // material index per mesh (-1 = none)
	DefaultScene int
}

// MeshMaterial returns the material index referenced by mesh i, or -1 if the
// mesh has no material or i is out of range.
func (a *GltfAsset) MeshMaterial(i int) int {
	if i < 0 || i >= len(a.meshMaterial) {
		return -1
	}
	return a.meshMaterial[i]
}

// Get resolves a sub-asset by its stable label (INV-4). The returned value is
// one of GltfScene, *mesh.Mesh, *material.Material, or renderimage.Image.
func (a *GltfAsset) Get(label GltfAssetLabel) (any, bool) {
	idx, ok := a.labels[label]
	if !ok {
		return nil, false
	}
	switch label.Kind {
	case KindScene:
		return a.Scenes[idx], true
	case KindMesh:
		return a.Meshes[idx], true
	case KindMaterial:
		return a.Materials[idx], true
	case KindTexture:
		return a.Textures[idx], true
	}
	return nil, false
}

// Labels returns every sub-asset label in deterministic (kind, index) order.
func (a *GltfAsset) Labels() []GltfAssetLabel {
	out := make([]GltfAssetLabel, 0, len(a.labels))
	for l := range a.labels {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Index < out[j].Index
	})
	return out
}

// GltfLoader decodes .gltf (JSON) and .glb (binary) files into a GltfAsset.
type GltfLoader struct{}

var _ asset.AssetLoader[GltfAsset, LoadSettings] = GltfLoader{}

// Extensions reports the file extensions this loader handles.
func (GltfLoader) Extensions() []string { return []string{".gltf", ".glb"} }

// Load reads all of r and decodes it into a GltfAsset (INV-2: full asset or error).
func (GltfLoader) Load(r io.Reader, _ LoadSettings) (GltfAsset, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return GltfAsset{}, fmt.Errorf("gltf: read input: %w", err)
	}
	return Decode(raw)
}

// RegisterAll registers the glTF loader with the given AssetServer.
func RegisterAll(srv *asset.AssetServer) error {
	if srv == nil {
		return errors.New("asset/formats/gltf: nil AssetServer")
	}
	asset.RegisterLoader(srv, GltfLoader{})
	return nil
}

// Decode parses glTF bytes — auto-detecting JSON (.gltf) vs binary (.glb) — into
// a GltfAsset. It never panics on malformed input: a recover guard converts any
// unexpected panic into an error, so a partial asset never escapes (INV-2).
func Decode(raw []byte) (a GltfAsset, err error) {
	defer func() {
		if r := recover(); r != nil {
			a = GltfAsset{}
			err = fmt.Errorf("gltf: recovered from panic: %v", r)
		}
	}()

	jsonBytes, glbBin, err := splitContainer(raw)
	if err != nil {
		return GltfAsset{}, err
	}
	var doc document
	if err := json.Unmarshal(jsonBytes, &doc); err != nil {
		return GltfAsset{}, fmt.Errorf("gltf: parse JSON: %w", err)
	}
	d := &decoder{doc: &doc}
	if err := d.resolveBuffers(glbBin); err != nil {
		return GltfAsset{}, err
	}
	return d.build()
}

// GLB binary container constants (little-endian magic + chunk type tags).
const (
	glbMagic     = 0x46546C67 // "glTF"
	glbChunkJSON = 0x4E4F534A // "JSON"
	glbChunkBIN  = 0x004E4942 // "BIN\0"
)

// splitContainer detects GLB vs plain-JSON glTF. For GLB it returns the JSON
// chunk and the BIN chunk; for plain JSON it returns the bytes verbatim, nil BIN.
func splitContainer(raw []byte) (jsonBytes, glbBin []byte, err error) {
	if len(raw) >= 4 && binary.LittleEndian.Uint32(raw[:4]) == glbMagic {
		return parseGLB(raw)
	}
	return raw, nil, nil
}

// parseGLB validates the 12-byte GLB header and walks its chunks.
func parseGLB(raw []byte) (jsonBytes, glbBin []byte, err error) {
	if len(raw) < 12 {
		return nil, nil, fmt.Errorf("%w: header truncated", ErrInvalidGLB)
	}
	if version := binary.LittleEndian.Uint32(raw[4:8]); version != 2 {
		return nil, nil, fmt.Errorf("%w: unsupported version %d", ErrInvalidGLB, version)
	}
	if length := binary.LittleEndian.Uint32(raw[8:12]); int(length) != len(raw) {
		return nil, nil, fmt.Errorf("%w: declared length %d != actual %d", ErrInvalidGLB, length, len(raw))
	}

	for off := 12; off+8 <= len(raw); {
		clen := binary.LittleEndian.Uint32(raw[off : off+4])
		ctype := binary.LittleEndian.Uint32(raw[off+4 : off+8])
		off += 8
		if off+int(clen) > len(raw) {
			return nil, nil, fmt.Errorf("%w: chunk length %d overflows file", ErrInvalidGLB, clen)
		}
		chunk := raw[off : off+int(clen)]
		off += int(clen)
		switch ctype {
		case glbChunkJSON:
			jsonBytes = chunk
		case glbChunkBIN:
			glbBin = chunk
		}
	}
	if jsonBytes == nil {
		return nil, nil, fmt.Errorf("%w: missing JSON chunk", ErrInvalidGLB)
	}
	return jsonBytes, glbBin, nil
}

// build performs the fan-out: it converts every sub-asset in glTF declaration
// order and records its stable label (INV-4). Any conversion error aborts the
// whole load so nothing partial is returned (INV-2).
func (d *decoder) build() (GltfAsset, error) {
	a := GltfAsset{labels: make(map[GltfAssetLabel]int)}

	// Meshes: one mesh.Mesh per glTF primitive, in declaration order.
	for _, gm := range d.doc.Meshes {
		for _, p := range gm.Primitives {
			m, err := d.buildPrimitive(p)
			if err != nil {
				return GltfAsset{}, err
			}
			matIdx := -1
			if p.Material != nil {
				matIdx = int(*p.Material)
			}
			a.labels[GltfAssetLabel{Kind: KindMesh, Index: uint32(len(a.Meshes))}] = len(a.Meshes)
			a.Meshes = append(a.Meshes, m)
			a.meshMaterial = append(a.meshMaterial, matIdx)
		}
	}
	// Materials.
	for i, gmat := range d.doc.Materials {
		a.labels[GltfAssetLabel{Kind: KindMaterial, Index: uint32(i)}] = i
		a.Materials = append(a.Materials, buildMaterial(gmat))
	}
	// Textures (embedded images only).
	for i, gt := range d.doc.Textures {
		img, err := d.buildTexture(gt)
		if err != nil {
			return GltfAsset{}, err
		}
		a.labels[GltfAssetLabel{Kind: KindTexture, Index: uint32(i)}] = i
		a.Textures = append(a.Textures, img)
	}
	// Nodes (hierarchy) + scenes (node roots).
	for _, gn := range d.doc.Nodes {
		meshIdx := -1
		if gn.Mesh != nil {
			meshIdx = int(*gn.Mesh)
		}
		a.Nodes = append(a.Nodes, GltfNode{Name: gn.Name, Mesh: meshIdx, Children: toIntSlice(gn.Children)})
	}
	for i, gs := range d.doc.Scenes {
		a.labels[GltfAssetLabel{Kind: KindScene, Index: uint32(i)}] = i
		a.Scenes = append(a.Scenes, GltfScene{Name: gs.Name, Nodes: toIntSlice(gs.Nodes)})
	}
	if d.doc.Scene != nil {
		a.DefaultScene = int(*d.doc.Scene)
	}
	return a, nil
}

func toIntSlice(in []uint32) []int {
	if len(in) == 0 {
		return nil
	}
	out := make([]int, len(in))
	for i, v := range in {
		out[i] = int(v)
	}
	return out
}
