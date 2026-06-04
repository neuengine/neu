package gltf

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	pkgmath "github.com/neuengine/neu/pkg/math"
	renderimage "github.com/neuengine/neu/pkg/render/image"
	"github.com/neuengine/neu/pkg/render/material"
	"github.com/neuengine/neu/pkg/render/mesh"
)

// decoder holds a parsed document plus its resolved buffer bytes. All per-file
// conversion methods hang off it so buffers are resolved exactly once.
type decoder struct {
	doc     *document
	buffers [][]byte // resolved bytes, index-aligned with doc.Buffers
}

// componentSize returns the byte width of one accessor component.
func componentSize(ct int) (int, error) {
	switch ct {
	case compByte, compUnsignedByte:
		return 1, nil
	case compShort, compUnsignedShort:
		return 2, nil
	case compUnsignedInt, compFloat:
		return 4, nil
	}
	return 0, fmt.Errorf("gltf: unknown componentType %d", ct)
}

// typeComponentCount returns the number of components for an accessor type.
func typeComponentCount(t string) (int, error) {
	switch t {
	case "SCALAR":
		return 1, nil
	case "VEC2":
		return 2, nil
	case "VEC3":
		return 3, nil
	case "VEC4":
		return 4, nil
	case "MAT2":
		return 4, nil
	case "MAT3":
		return 9, nil
	case "MAT4":
		return 16, nil
	}
	return 0, fmt.Errorf("gltf: unknown accessor type %q", t)
}

// resolveBuffers decodes every glTF buffer once. glbBin is the GLB BIN chunk
// (nil for plain-JSON .gltf) and backs the uri-less buffer 0.
func (d *decoder) resolveBuffers(glbBin []byte) error {
	d.buffers = make([][]byte, len(d.doc.Buffers))
	for i, b := range d.doc.Buffers {
		data, err := decodeBuffer(b, glbBin)
		if err != nil {
			return fmt.Errorf("gltf: buffer %d: %w", i, err)
		}
		if uint32(len(data)) < b.ByteLength {
			return fmt.Errorf("gltf: buffer %d: have %d bytes, need %d", i, len(data), b.ByteLength)
		}
		d.buffers[i] = data
	}
	return nil
}

func decodeBuffer(b buffer, glbBin []byte) ([]byte, error) {
	if b.URI == "" {
		if glbBin == nil {
			return nil, errors.New("buffer has no uri and no GLB BIN chunk present")
		}
		return glbBin, nil
	}
	if strings.HasPrefix(b.URI, "data:") {
		return decodeDataURI(b.URI)
	}
	// An external file reference needs the AssetServer/VFS (App integration).
	return nil, fmt.Errorf("%w: %q", ErrExternalResource, b.URI)
}

// decodeDataURI decodes a base64 "data:...;base64,<payload>" URI.
func decodeDataURI(uri string) ([]byte, error) {
	_, payload, found := strings.Cut(uri, ";base64,")
	if !found {
		return nil, fmt.Errorf("%w: only base64 data URIs are supported", ErrUnsupportedDataURI)
	}
	data, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("gltf: base64 data URI: %w", err)
	}
	return data, nil
}

// accessorData reads accessor i and returns its elements de-interleaved into a
// tightly packed slice, plus the component type and per-element component count.
// Every buffer index is bounds-checked so malformed glTF errors rather than panics.
func (d *decoder) accessorData(i uint32) (data []byte, compType, compCount int, count uint32, err error) {
	if int(i) >= len(d.doc.Accessors) {
		return nil, 0, 0, 0, fmt.Errorf("gltf: accessor index %d out of range", i)
	}
	a := d.doc.Accessors[i]
	cs, err := componentSize(a.ComponentType)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	cc, err := typeComponentCount(a.Type)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	elemSize := cs * cc

	// A bufferView-less accessor is zero-initialised (sparse data is not yet read).
	if a.BufferView == nil {
		return make([]byte, int(a.Count)*elemSize), a.ComponentType, cc, a.Count, nil
	}
	if int(*a.BufferView) >= len(d.doc.BufferViews) {
		return nil, 0, 0, 0, fmt.Errorf("gltf: bufferView index %d out of range", *a.BufferView)
	}
	bv := d.doc.BufferViews[*a.BufferView]
	if int(bv.Buffer) >= len(d.buffers) {
		return nil, 0, 0, 0, fmt.Errorf("gltf: buffer index %d out of range", bv.Buffer)
	}
	buf := d.buffers[bv.Buffer]
	base := int(bv.ByteOffset) + int(a.ByteOffset)
	stride := elemSize
	if bv.ByteStride != nil && *bv.ByteStride != 0 {
		stride = int(*bv.ByteStride)
	}
	if a.Count > 0 {
		lastEnd := base + (int(a.Count)-1)*stride + elemSize
		if base < 0 || lastEnd > len(buf) {
			return nil, 0, 0, 0, fmt.Errorf("gltf: accessor %d reads past buffer end (%d > %d)", i, lastEnd, len(buf))
		}
	}
	out := make([]byte, int(a.Count)*elemSize)
	for e := range int(a.Count) {
		src := base + e*stride
		copy(out[e*elemSize:(e+1)*elemSize], buf[src:src+elemSize])
	}
	return out, a.ComponentType, cc, a.Count, nil
}

// bufferViewBytes returns the raw bytes of a buffer view (used for embedded images).
func (d *decoder) bufferViewBytes(i uint32) ([]byte, error) {
	if int(i) >= len(d.doc.BufferViews) {
		return nil, fmt.Errorf("gltf: bufferView index %d out of range", i)
	}
	bv := d.doc.BufferViews[i]
	if int(bv.Buffer) >= len(d.buffers) {
		return nil, fmt.Errorf("gltf: buffer index %d out of range", bv.Buffer)
	}
	buf := d.buffers[bv.Buffer]
	start := int(bv.ByteOffset)
	end := start + int(bv.ByteLength)
	if start < 0 || end > len(buf) {
		return nil, fmt.Errorf("gltf: bufferView %d out of buffer bounds", i)
	}
	return buf[start:end], nil
}

// buildPrimitive converts one glTF primitive into a validated mesh.Mesh.
func (d *decoder) buildPrimitive(p primitive) (*mesh.Mesh, error) {
	m := mesh.NewMesh(topology(p.Mode))

	// POSITION is mandatory (mesh INV-1).
	posIdx, ok := p.Attributes["POSITION"]
	if !ok {
		return nil, mesh.ErrMeshNoPosition
	}
	if err := d.addVertexAttr(m, mesh.AttrPosition, mesh.FormatFloat32x3, posIdx); err != nil {
		return nil, err
	}
	// Optional float-typed attributes, in a fixed order for determinism.
	optional := []struct {
		name   string
		kind   mesh.AttrKind
		format mesh.VertexFormat
	}{
		{"NORMAL", mesh.AttrNormal, mesh.FormatFloat32x3},
		{"TEXCOORD_0", mesh.AttrUV0, mesh.FormatFloat32x2},
		{"TEXCOORD_1", mesh.AttrUV1, mesh.FormatFloat32x2},
		{"TANGENT", mesh.AttrTangent, mesh.FormatFloat32x4},
		{"COLOR_0", mesh.AttrColor, mesh.FormatFloat32x4},
		{"WEIGHTS_0", mesh.AttrJointWeights, mesh.FormatFloat32x4}, // float skin weights (T-6 skin foundation)
	}
	for _, oa := range optional {
		if idx, ok := p.Attributes[oa.name]; ok {
			if err := d.addVertexAttr(m, oa.kind, oa.format, idx); err != nil {
				return nil, err
			}
		}
	}
	// JOINTS_0 carries integer joint indices (unsigned byte/short), so it needs a
	// dedicated path rather than the FLOAT-only addVertexAttr.
	if idx, ok := p.Attributes["JOINTS_0"]; ok {
		if err := d.addJointIndices(m, idx); err != nil {
			return nil, err
		}
	}
	if p.Indices != nil {
		ib, err := d.buildIndices(*p.Indices)
		if err != nil {
			return nil, err
		}
		m.SetIndices(ib)
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("gltf: primitive mesh invalid: %w", err)
	}
	return m, nil
}

// addVertexAttr reads a FLOAT vertex attribute and attaches it to the mesh.
// glTF stores positions/normals/uvs/tangents as FLOAT; non-float vertex streams
// (e.g. normalised byte colours) are not yet supported and produce a clear error.
func (d *decoder) addVertexAttr(m *mesh.Mesh, kind mesh.AttrKind, format mesh.VertexFormat, accessorIdx uint32) error {
	data, compType, _, count, err := d.accessorData(accessorIdx)
	if err != nil {
		return err
	}
	if compType != compFloat {
		return fmt.Errorf("gltf: attribute accessor %d: only FLOAT components are supported, got %d", accessorIdx, compType)
	}
	if uint32(len(data)) != count*format.Size() {
		return fmt.Errorf("gltf: attribute accessor %d: element size mismatch for mesh format", accessorIdx)
	}
	m.SetAttribute(mesh.VertexAttribute{Kind: kind, Format: format, Data: data})
	return nil
}

// addJointIndices reads a JOINTS_0 accessor (VEC4 of unsigned byte or short) and
// attaches it as a uint16x4 joint-index stream, widening 8-bit indices to 16-bit
// (the mesh package's joint-index width). Skin weights (WEIGHTS_0) go through the
// FLOAT optional path; the runtime skinning/skeleton is a later (Phase 7) concern.
func (d *decoder) addJointIndices(m *mesh.Mesh, accessorIdx uint32) error {
	data, compType, compCount, count, err := d.accessorData(accessorIdx)
	if err != nil {
		return err
	}
	if compCount != 4 {
		return fmt.Errorf("gltf: JOINTS_0 accessor %d must be VEC4", accessorIdx)
	}
	var out []byte
	switch compType {
	case compUnsignedShort:
		out = data // already uint16x4, tightly packed
	case compUnsignedByte:
		out = make([]byte, int(count)*8) // 4 components × 2 bytes
		for i := range int(count) * 4 {
			out[i*2] = data[i] // low byte = value; high byte stays 0 (LE)
		}
	default:
		return fmt.Errorf("gltf: JOINTS_0 accessor %d: unsupported componentType %d (want unsigned byte/short)", accessorIdx, compType)
	}
	m.SetAttribute(mesh.VertexAttribute{Kind: mesh.AttrJointIndices, Format: mesh.FormatUint16x4, Data: out})
	return nil
}

// buildIndices converts an index accessor into a mesh.IndexBuffer, widening
// 8-bit indices to 16-bit (the mesh package's narrowest index width).
func (d *decoder) buildIndices(accessorIdx uint32) (mesh.IndexBuffer, error) {
	data, compType, compCount, count, err := d.accessorData(accessorIdx)
	if err != nil {
		return mesh.IndexBuffer{}, err
	}
	if compCount != 1 {
		return mesh.IndexBuffer{}, fmt.Errorf("gltf: index accessor %d must be SCALAR", accessorIdx)
	}
	switch compType {
	case compUnsignedShort:
		return mesh.IndexBuffer{Data: data, Wide: false}, nil
	case compUnsignedInt:
		return mesh.IndexBuffer{Data: data, Wide: true}, nil
	case compUnsignedByte:
		wide := make([]byte, int(count)*2)
		for i := range int(count) {
			wide[i*2] = data[i] // low byte; high byte stays zero (LE)
		}
		return mesh.IndexBuffer{Data: wide, Wide: false}, nil
	}
	return mesh.IndexBuffer{}, fmt.Errorf("gltf: index accessor %d: unsupported componentType %d", accessorIdx, compType)
}

// topology maps a glTF primitive mode to a mesh topology (TRIANGLES when absent).
func topology(mode *uint32) mesh.PrimitiveTopology {
	if mode == nil {
		return mesh.TopologyTriangleList
	}
	switch *mode {
	case modePoints:
		return mesh.TopologyPointList
	case modeLines:
		return mesh.TopologyLineList
	case modeLineStrip, modeLineLoop:
		return mesh.TopologyLineStrip
	case modeTriangleStrip:
		return mesh.TopologyTriangleStrip
	case modeTriangles, modeTriangleFan:
		return mesh.TopologyTriangleList
	}
	return mesh.TopologyTriangleList
}

// buildMaterial converts a glTF material into a PBR metallic-roughness Material.
// Factors are fully mapped; texture bindings need asset handles and are deferred
// to App integration (the texture sub-assets are still fanned out and addressable).
func buildMaterial(gm gltfMaterial) *material.Material {
	m := material.StandardPBR()
	if gm.PBR != nil {
		if len(gm.PBR.BaseColorFactor) == 4 {
			f := gm.PBR.BaseColorFactor
			m.SetColor("base_color", pkgmath.LinearRgba{R: f[0], G: f[1], B: f[2], A: f[3]})
		}
		if gm.PBR.MetallicFactor != nil {
			m.SetFloat("metallic", *gm.PBR.MetallicFactor)
		}
		if gm.PBR.RoughnessFactor != nil {
			m.SetFloat("roughness", *gm.PBR.RoughnessFactor)
		}
	}
	if len(gm.Emissive) == 3 {
		m.SetColor("emissive", pkgmath.LinearRgba{R: gm.Emissive[0], G: gm.Emissive[1], B: gm.Emissive[2], A: 1})
	}
	switch gm.AlphaMode {
	case "MASK":
		m.Alpha = material.AlphaMask
	case "BLEND":
		m.Alpha = material.AlphaBlend
	default:
		m.Alpha = material.AlphaOpaque
	}
	m.DoubleSided = gm.DoubleSided
	return m
}

// buildTexture decodes an embedded glTF texture (data-URI or buffer-view image)
// into an RGBA8 image. External-URI images need the VFS and are not supported here.
func (d *decoder) buildTexture(gt gltfTexture) (renderimage.Image, error) {
	if gt.Source == nil || int(*gt.Source) >= len(d.doc.Images) {
		return renderimage.Image{}, errors.New("gltf: texture source out of range")
	}
	img := d.doc.Images[*gt.Source]

	var raw []byte
	switch {
	case img.BufferView != nil:
		b, err := d.bufferViewBytes(*img.BufferView)
		if err != nil {
			return renderimage.Image{}, err
		}
		raw = b
	case strings.HasPrefix(img.URI, "data:"):
		b, err := decodeDataURI(img.URI)
		if err != nil {
			return renderimage.Image{}, err
		}
		raw = b
	default:
		return renderimage.Image{}, fmt.Errorf("%w: external image %q", ErrExternalResource, img.URI)
	}

	decoded, err := renderimage.DecodeImage(bytes.NewReader(raw), img.MimeType)
	if err != nil {
		return renderimage.Image{}, fmt.Errorf("gltf: decode texture: %w", err)
	}
	return *decoded, nil
}
