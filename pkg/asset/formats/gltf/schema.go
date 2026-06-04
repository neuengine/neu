package gltf

// glTF 2.0 accessor component types (the OpenGL constant values).
const (
	compByte          = 5120
	compUnsignedByte  = 5121
	compShort         = 5122
	compUnsignedShort = 5123
	compUnsignedInt   = 5125
	compFloat         = 5126
)

// glTF 2.0 primitive topology modes.
const (
	modePoints        = 0
	modeLines         = 1
	modeLineLoop      = 2
	modeLineStrip     = 3
	modeTriangles     = 4
	modeTriangleStrip = 5
	modeTriangleFan   = 6
)

// document is the parsed glTF 2.0 JSON document (the subset the loader consumes).
// Fields the engine does not yet map (cameras, texture samplers, sparse
// accessors, skins, morph targets) are intentionally omitted — unknown JSON keys
// are ignored.
type document struct {
	Asset       assetInfo       `json:"asset"`
	Scene       *uint32         `json:"scene"`
	Scenes      []gltfScene     `json:"scenes"`
	Nodes       []gltfNode      `json:"nodes"`
	Meshes      []gltfMesh      `json:"meshes"`
	Materials   []gltfMaterial  `json:"materials"`
	Accessors   []accessor      `json:"accessors"`
	BufferViews []bufferView    `json:"bufferViews"`
	Buffers     []buffer        `json:"buffers"`
	Images      []gltfImage     `json:"images"`
	Textures    []gltfTexture   `json:"textures"`
	Animations  []gltfAnimation `json:"animations"`
}

// gltfAnimation is one glTF animation: a set of channels (target node + property)
// driven by samplers (keyframe input/output accessors).
type gltfAnimation struct {
	Name     string        `json:"name"`
	Channels []animChannel `json:"channels"`
	Samplers []animSampler `json:"samplers"`
}

type animChannel struct {
	Target  animTarget `json:"target"`
	Sampler uint32     `json:"sampler"`
}

type animTarget struct {
	Node *uint32 `json:"node"`
	Path string  `json:"path"` // "translation" | "rotation" | "scale" | "weights"
}

type animSampler struct {
	Interpolation string `json:"interpolation"` // "LINEAR" (default) | "STEP" | "CUBICSPLINE"
	Input         uint32 `json:"input"`         // keyframe-time accessor (SCALAR FLOAT)
	Output        uint32 `json:"output"`        // keyframe-value accessor (VEC3/VEC4 FLOAT)
}

type assetInfo struct {
	Version   string `json:"version"`
	Generator string `json:"generator"`
}

type gltfScene struct {
	Name  string   `json:"name"`
	Nodes []uint32 `json:"nodes"`
}

type gltfNode struct {
	Name     string   `json:"name"`
	Mesh     *uint32  `json:"mesh"`
	Children []uint32 `json:"children"`
}

type gltfMesh struct {
	Name       string      `json:"name"`
	Primitives []primitive `json:"primitives"`
}

type primitive struct {
	Attributes map[string]uint32 `json:"attributes"`
	Indices    *uint32           `json:"indices"`
	Material   *uint32           `json:"material"`
	Mode       *uint32           `json:"mode"`
}

type gltfMaterial struct {
	PBR         *pbrMetallicRough `json:"pbrMetallicRoughness"`
	AlphaCutoff *float32          `json:"alphaCutoff"`
	Name        string            `json:"name"`
	AlphaMode   string            `json:"alphaMode"`
	Emissive    []float32         `json:"emissiveFactor"`
	DoubleSided bool              `json:"doubleSided"`
}

type pbrMetallicRough struct {
	MetallicFactor   *float32    `json:"metallicFactor"`
	RoughnessFactor  *float32    `json:"roughnessFactor"`
	BaseColorTexture *textureRef `json:"baseColorTexture"`
	BaseColorFactor  []float32   `json:"baseColorFactor"`
}

type textureRef struct {
	Index    uint32 `json:"index"`
	TexCoord uint32 `json:"texCoord"`
}

type accessor struct {
	BufferView    *uint32 `json:"bufferView"`
	Type          string  `json:"type"`
	ComponentType int     `json:"componentType"`
	ByteOffset    uint32  `json:"byteOffset"`
	Count         uint32  `json:"count"`
	Normalized    bool    `json:"normalized"`
}

type bufferView struct {
	ByteStride *uint32 `json:"byteStride"`
	Buffer     uint32  `json:"buffer"`
	ByteOffset uint32  `json:"byteOffset"`
	ByteLength uint32  `json:"byteLength"`
}

type buffer struct {
	URI        string `json:"uri"`
	ByteLength uint32 `json:"byteLength"`
}

type gltfImage struct {
	BufferView *uint32 `json:"bufferView"`
	Name       string  `json:"name"`
	URI        string  `json:"uri"`
	MimeType   string  `json:"mimeType"`
}

type gltfTexture struct {
	Source  *uint32 `json:"source"`
	Sampler *uint32 `json:"sampler"`
	Name    string  `json:"name"`
}
