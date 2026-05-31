// Package gltf decodes glTF 2.0 assets (.gltf JSON and .glb binary) into the
// engine's geometry and material types. A single file fans out into many
// sub-assets — meshes, materials, textures, scenes — each addressed by a
// deterministic [GltfAssetLabel] derived from glTF declaration order, so the
// addressing is identical across reloads (l2-asset-formats-go INV-4).
//
// The loader is stateless and never panics on malformed input: every failure
// returns a wrapped error and no partial asset enters the store (INV-2).
//
// Bootstrap: l2-asset-formats-go Draft. Animations, skins, morph targets, the
// scene.DynamicScene conversion, and external-URI resource resolution are
// deferred to App integration (they require the World / AssetServer / VFS).
package gltf

import "strconv"

// GltfKind classifies the category of a sub-asset produced by a glTF fan-out.
type GltfKind uint8

const (
	KindScene       GltfKind = iota // node-root scene record
	KindMesh                        // one mesh.Mesh per glTF primitive
	KindMaterial                    // PBR metallic-roughness material
	KindTexture                     // decoded embedded image
	KindAnimation                   // reserved — deferred to App integration
	KindSkin                        // reserved — deferred to App integration
	KindMorphTarget                 // reserved — deferred to App integration
)

// String returns the canonical glTF kind name used in labels (INV-4).
func (k GltfKind) String() string {
	switch k {
	case KindScene:
		return "Scene"
	case KindMesh:
		return "Mesh"
	case KindMaterial:
		return "Material"
	case KindTexture:
		return "Texture"
	case KindAnimation:
		return "Animation"
	case KindSkin:
		return "Skin"
	case KindMorphTarget:
		return "MorphTarget"
	}
	return "Unknown"
}

// GltfAssetLabel addresses one sub-asset within a loaded glTF by (kind, index).
// The index is the sub-asset's position in glTF declaration order, so the label
// resolves to the same sub-asset every time the file is reloaded (INV-4):
// Mesh(0) is always the first primitive declared.
type GltfAssetLabel struct {
	Kind  GltfKind
	Index uint32
}

// String renders the label in canonical "Kind(index)" form, e.g. "Mesh(0)".
func (l GltfAssetLabel) String() string {
	return l.Kind.String() + "(" + strconv.FormatUint(uint64(l.Index), 10) + ")"
}
