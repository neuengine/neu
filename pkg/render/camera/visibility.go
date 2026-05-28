package camera

// Visibility is the user-facing explicit visibility setting on an entity.
// It does not represent the computed result — see [InheritedVisibility] and
// [ViewVisibility] for those.
type Visibility uint8

const (
	// VisibilityInherited inherits the parent's [InheritedVisibility] (default).
	VisibilityInherited Visibility = iota
	// VisibilityHidden forces the entity and all its descendants invisible.
	VisibilityHidden
	// VisibilityVisible forces the entity to be considered visible regardless
	// of its parent's state (but frustum cull still applies).
	VisibilityVisible
)

// InheritedVisibility is a computed component: true when the entity is visible
// according to the hierarchy walk. Updated by propagateVisibility each frame.
// Read-only for user code.
type InheritedVisibility struct{ Visible bool }

// ViewVisibility is a computed component: true when the entity is both
// hierarchy-visible (InheritedVisibility) AND passes the camera frustum cull.
// This is the sole gate for inclusion in the draw queue (INV-1 of L1 spec).
// Updated by cullFrustum each frame. Read-only for user code.
type ViewVisibility struct{ Visible bool }
