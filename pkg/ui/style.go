// Package ui provides the entity-driven UI layer: a Style component holds
// flexbox layout properties, a solver writes a LayoutRect per node, Interaction
// state is computed by hit-testing, and a UiFeature composites UI last. No CSS
// cascade — every Style value is explicit on its entity.
//
// Bootstrap: l2-ui-system-go Draft (Phase 6 Track D, C29 gate open).
package ui

// ValKind tags how a Val resolves to pixels.
type ValKind uint8

const (
	// ValAuto sizes from content / flex rules.
	ValAuto ValKind = iota
	// ValPx is an absolute pixel length.
	ValPx
	// ValPercent is a percentage of the parent's corresponding dimension.
	ValPercent
	// ValVw is a percentage of the viewport width.
	ValVw
	// ValVh is a percentage of the viewport height.
	ValVh
)

// Val is a length value: a kind plus a magnitude (ignored for ValAuto).
type Val struct {
	Kind  ValKind
	Value float32
}

// Px / Percent / Vw / Vh are convenience constructors.
func Px(v float32) Val      { return Val{Kind: ValPx, Value: v} }
func Percent(v float32) Val { return Val{Kind: ValPercent, Value: v} }
func Vw(v float32) Val      { return Val{Kind: ValVw, Value: v} }
func Vh(v float32) Val      { return Val{Kind: ValVh, Value: v} }

// Auto is the zero Val (content-sized).
var Auto = Val{Kind: ValAuto}

// Resolve converts a Val to pixels given the parent dimension and viewport size.
// The bool is false for ValAuto (the caller decides the auto size).
func (v Val) Resolve(parent, vw, vh float32) (float32, bool) {
	switch v.Kind {
	case ValPx:
		return v.Value, true
	case ValPercent:
		return parent * v.Value / 100, true
	case ValVw:
		return vw * v.Value / 100, true
	case ValVh:
		return vh * v.Value / 100, true
	default: // ValAuto
		return 0, false
	}
}

// Display selects the layout algorithm for a node's children.
type Display uint8

const (
	DisplayFlex Display = iota
	DisplayNone
)

// FlexDirection is the main axis of a flex container.
type FlexDirection uint8

const (
	Row FlexDirection = iota
	Column
	RowReverse
	ColumnReverse
)

// IsRow reports whether the main axis is horizontal.
func (d FlexDirection) IsRow() bool { return d == Row || d == RowReverse }

// IsReverse reports whether children are laid out in reverse order.
func (d FlexDirection) IsReverse() bool { return d == RowReverse || d == ColumnReverse }

// JustifyContent distributes free space along the main axis.
type JustifyContent uint8

const (
	JustifyFlexStart JustifyContent = iota
	JustifyCenter
	JustifyFlexEnd
	JustifySpaceBetween
	JustifySpaceAround
	JustifySpaceEvenly
)

// AlignItems positions children along the cross axis.
type AlignItems uint8

const (
	AlignStretch AlignItems = iota
	AlignFlexStart
	AlignCenter
	AlignFlexEnd
)

// UiRect is a four-sided length (margin, padding, border).
type UiRect struct {
	Left, Right, Top, Bottom Val
}

// PxRect is a UiRect with all sides in pixels.
func PxRect(l, r, t, b float32) UiRect {
	return UiRect{Left: Px(l), Right: Px(r), Top: Px(t), Bottom: Px(b)}
}

// Style is the per-node layout component (L1 §4.2). Defaults (the zero value)
// are a content-sized flex column at flex-start/stretch.
type Style struct {
	Display        Display
	FlexDirection  FlexDirection
	JustifyContent JustifyContent
	AlignItems     AlignItems
	Width, Height  Val
	MinWidth       Val
	MinHeight      Val
	MaxWidth       Val
	MaxHeight      Val
	Margin         UiRect
	Padding        UiRect
	Gap            float32 // pixel gap between children along the main axis
	FlexGrow       float32
	FlexShrink     float32
	FlexBasis      Val
}
