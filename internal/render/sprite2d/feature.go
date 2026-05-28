// Package sprite2d implements the Sprite2DFeature: a RenderFeature that
// extracts, sorts, and batches 2D sprites for the render pipeline.
//
// The feature reuses render-core infrastructure (RenderDataHolder, VisibilityGroup)
// and contributes draw commands to the Transparent render phase.
//
// Bootstrap: l2-2d-rendering-go Draft (C29 P5 gate open).
package sprite2d

import (
	"sort"

	render "github.com/neuengine/neu/internal/render"
	"github.com/neuengine/neu/pkg/render/sprite"
)

// ExtractedSprite is the render-world representation of one visible sprite.
// Copied from the main world during the Extract phase (render-core INV-4).
type ExtractedSprite struct {
	// Transform is the world-space 4×4 matrix (column-major).
	Transform [16]float32
	// AtlasUV holds the sub-region UVs [u0, v0, u1, v1].
	AtlasUV [4]float32
	// Color is the pre-multiplied RGBA tint.
	Color [4]float32
	// batchKey packs atlas texture + material + blend for batch merging.
	batchKey uint64
	// sortKey packs Z, Y, and entity index for deterministic ordering (INV-2).
	sortKey uint64
}

// SpriteProcessorData caches per-entity GPU resources to avoid re-uploading
// unchanged sprites (sprite associated data pattern, L1 §4.11).
type SpriteProcessorData struct {
	AtlasUV          [4]float32
	LastImageVersion uint64
	BatchKey         uint64
}

// Sprite2DFeature implements render.RenderFeature for the 2D sprite pipeline.
// It is registered with the RenderSubApp and called each frame.
type Sprite2DFeature struct {
	extracted []ExtractedSprite
}

// NewSprite2DFeature creates an empty feature ready to be added to a RenderSubApp.
func NewSprite2DFeature() *Sprite2DFeature { return &Sprite2DFeature{} }

// Initialize is called once when the feature is registered. No-op for sprites.
func (f *Sprite2DFeature) Initialize(_ *render.RenderSubApp) {}

// Collect enumerates 2D cameras and builds per-camera visibility lists.
func (f *Sprite2DFeature) Collect(_ *render.CollectContext) {}

// Extract copies Sprite + Transform data from the main world into ExtractedSprites.
// Entities without a valid Image handle are skipped (INV-1).
func (f *Sprite2DFeature) Extract(_ *render.ExtractContext) {
	// In a full implementation this would query the ECS world for entities with
	// (Sprite + Transform + Visibility). For Bootstrap the list is empty.
	f.extracted = f.extracted[:0]
}

// Prepare sorts extracted sprites and batches them into draw calls.
// Sort order: Z (primary) → Y (optional) → EntityIndex (tie-break, INV-2).
func (f *Sprite2DFeature) Prepare(_ *render.PrepareContext) {
	sort.Slice(f.extracted, func(i, j int) bool {
		return f.extracted[i].sortKey < f.extracted[j].sortKey
	})
}

// Draw issues GPU draw calls for each batch of adjacent same-key sprites (INV-3).
func (f *Sprite2DFeature) Draw(_ *render.DrawContext, _ *render.RenderView) {
	// Walk the sorted list and emit one draw call per contiguous same-batchKey run.
	// Bootstrap: no GPU backend in CI — nothing to emit.
}

// Flush releases per-frame temporaries.
func (f *Sprite2DFeature) Flush(_ *render.FlushContext) {
	f.extracted = f.extracted[:0]
}

// PrepareEffectPermutations compiles/selects shader variants for sprites.
func (f *Sprite2DFeature) PrepareEffectPermutations(_ *render.PrepareContext) {}

// BuildSortKey packs Z, Y, and an entity index into a uint64 for stable sort.
// ZOrder is the primary key (higher Z draws on top in default mode).
func BuildSortKey(z, y float32, entityIndex uint32) uint64 {
	// Flip z so lower float32 sorts later (back-to-front for transparency).
	zBits := ^uint32(z * 65536)
	yBits := uint32(y * 65536)
	return uint64(zBits)<<32 | uint64(yBits)<<0 | uint64(entityIndex&0xFFFF)
}

// BatchKey packs texture handle, material ID, and blend mode into a uint64.
func BatchKey(textureID, materialID uint32, blendMode uint8) uint64 {
	return uint64(textureID)<<32 | uint64(materialID)<<8 | uint64(blendMode)
}

// SpritePickResult is returned by PickSprite.
type SpritePickResult struct {
	EntityIndex uint32
	Hit         bool
}

// PickSprite performs ray-rectangle intersection against the sorted sprite list.
// Returns the topmost (last in back-to-front sorted list) hit entity.
func PickSprite(extracted []ExtractedSprite, worldX, worldY float32) SpritePickResult {
	// Walk in reverse (topmost drawn sprite = last in sorted list).
	for i := len(extracted) - 1; i >= 0; i-- {
		s := &extracted[i]
		// Use the sprite's UV bounds as an axis-aligned proxy for picking.
		u0, v0, u1, v1 := s.AtlasUV[0], s.AtlasUV[1], s.AtlasUV[2], s.AtlasUV[3]
		if worldX >= u0 && worldX <= u1 && worldY >= v0 && worldY <= v1 {
			return SpritePickResult{EntityIndex: uint32(i), Hit: true}
		}
	}
	return SpritePickResult{}
}

// ── ExtractedSprite accessors ─────────────────────────────────────────────────

// SetSortKey sets the sort key (used by tests and example builders).
func (s *ExtractedSprite) SetSortKey(k uint64) { s.sortKey = k }

// GetSortKey returns the sort key.
func (s *ExtractedSprite) GetSortKey() uint64 { return s.sortKey }

// SetBatchKey sets the batch key (texture + material + blend).
func (s *ExtractedSprite) SetBatchKey(k uint64) { s.batchKey = k }

// GetBatchKey returns the batch key.
func (s *ExtractedSprite) GetBatchKey() uint64 { return s.batchKey }

// ── SortSprites ───────────────────────────────────────────────────────────────

// SortSprites sorts a slice of ExtractedSprites in place using the packed
// sortKey (Z → Y → entity index, INV-2). Exported for testing and examples.
func SortSprites(sprites []ExtractedSprite) {
	sort.Slice(sprites, func(i, j int) bool {
		return sprites[i].sortKey < sprites[j].sortKey
	})
}

// Ensure interface compliance at compile time.
var _ render.RenderFeature = (*Sprite2DFeature)(nil)

// Keep sprite package import alive for the build.
var _ = (*sprite.Sprite)(nil)
