# UI System — Go Implementation

**Version:** 0.1.0
**Status:** Stable
**Layer:** go
**Implements:** [l1-ui-system.md](l1-ui-system.md)

## Overview

Go-level design for the entity-driven UI layer. Every UI element is an entity carrying a
`Style` component (flexbox fields, pure data); a flexbox solver writes a computed
`LayoutRect` per node, recomputing only dirty subtrees (change-detection on `Style` +
hierarchy → INV-1). `Interaction` state (hover/press) is computed in `PreUpdate` by
hit-testing the layout tree in reverse render order (INV-3). Text is shaped into glyph
quads cached in a `FontAtlas` that reuses the render-core shelf-pack `DynamicAtlas`
(INV-4). A `UiFeature` (RenderFeature) composites UI last, after all game rendering
(INV-2), using an orthographic projection matching the viewport.

## Related Specifications

- [l1-ui-system.md](l1-ui-system.md) — L1 concept specification (parent)
- [l2-hierarchy-system-go.md](l2-hierarchy-system-go.md) — `ChildOf`/`Children` drive the layout tree; reparent triggers relayout
- [l2-render-core-go.md](l2-render-core-go.md) — `UiFeature` reuses the `RenderFeature` interface + SoA extract; `DynamicAtlas` for glyphs
- [l2-2d-rendering-go.md](l2-2d-rendering-go.md) — shared quad-batching + `Text2D` world-space text path
- [l2-input-system-go.md](l2-input-system-go.md) — pointer position + button state feed `Interaction`; picking integration
- [l2-window-system-go.md](l2-window-system-go.md) — viewport dimensions drive logical-pixel layout + the UI ortho projection
- [l2-change-detection-go.md](l2-change-detection-go.md) — `Changed[Style]` ticks drive the dirty-flag relayout (INV-1)
- [l2-math-system-go.md](l2-math-system-go.md) — `Vec2`, `Rect2D`, `Color`, `Affine2` for layout + offset transform

## 1. Motivation

Modeling UI as ECS entities lets the same query/event/schedule machinery serve menus and
gameplay alike. The Go binding's job is to (1) wrap a flexbox solver behind the `Style`
component + a dirty-flag relayout system, (2) reuse render-core's atlas and feature
infrastructure for glyphs and compositing rather than a parallel renderer, and (3) drive
`Interaction` from the existing input/picking layer. No CSS cascade — every `Style` value
is explicit on its entity, matching the engine's data-explicit philosophy.

## 2. Constraints & Assumptions

- **Go 1.26.3+**: generics for query iteration; `golang.org/x/image/font` (or stdlib `image/font` candidate) for TTF/OTF shaping — an x/ dependency requires an ADR per C-003.
- **C-003**: glyph rasterization prefers stdlib (`golang.org/x/image/font/opentype` is the documented candidate; ADR pending). Atlas packing reuses render-core `DynamicAtlas` (already stdlib).
- **INV-1**: layout recomputes only when `Style` or hierarchy changed this tick — a dirty-flag system reads `Changed[Style]` + reparent events and re-lays-out only affected subtrees.
- **INV-2**: a single `UiFeature` runs after all game render features; it owns its ortho pass.
- **INV-3**: `Interaction` is updated in `PreUpdate`, before user `Update` systems; user code reads but never writes it.
- **INV-4**: a `(Font, size, glyph)` key resolves to a cached atlas entry; a miss rasterizes once and packs into the shelf atlas.
- Layout values are logical pixels; the render pass scales by the window scale factor.

## 3. Core Invariants

> [!NOTE]
> See [l1-ui-system.md §3](l1-ui-system.md) for the technology-agnostic invariants
> (INV-1…INV-4). Go-specific compliance is tabulated in §4.

## 4. Invariant Compliance

| L1 Invariant | Implementation |
| :--- | :--- |
| **INV-1**: layout recomputed only on Style/hierarchy change (dirty flag) | `layoutSystem` (in `PostUpdate`) queries `Changed[Style]` and consumes hierarchy-change events; it marks dirty subtree roots and re-solves only those. A node's `LayoutRect` and cached `minSize` are recomputed only when its dirty bit (or an ancestor's) is set; the dirty flag propagates upward via `notifyMinSizeChanged`. |
| **INV-2**: UI renders after all game rendering (separate pass) | `UiFeature` implements `render.RenderFeature` and is registered to run last in the render schedule; its `Draw` emits an ortho UI pass after every scene/2D feature. Z-order within the pass follows painter's-order DFS + `ZIndex`. |
| **INV-3**: interaction updated in PreUpdate before user systems | `interactionSystem` runs in `PreUpdate`; it hit-tests pointer position against `LayoutRect`s in reverse render order, writes `Interaction`, and dispatches bubbling events. User `Update` systems observe the already-updated state. |
| **INV-4**: text glyphs cached in a font atlas | `FontAtlas` keys a `map[glyphKey]atlasEntry` where `glyphKey = {FontID, sizePx, rune}`; a miss rasterizes the glyph and packs it via the render-core shelf-pack `DynamicAtlas`. Repeated text with the same font/size reuses entries; the atlas grows/rebuilds when full. |

## Go Package

```
pkg/ui/
  style.go         // Style component, Val (Px/Percent/Auto/Vw/Vh), UiRect, Size enums
  node.go          // Node marker, LayoutRect (computed), ZIndex, Overflow
  widgets.go       // Text, TextSection, ImageNode, Button, ScrollView convenience bundles
  visual.go        // BackgroundColor, BorderColor, BorderRadius, Outline, Gradient
  interaction.go   // Interaction, MouseFilter, Focused, TabIndex, FocusNeighbor
  transform.go     // OffsetTransform (visual-only, applied at render)
  font.go          // Font asset, TextAlignment, TextWrapping
  plugin.go        // UiPlugin: registers layout/interaction/render systems + schedules
internal/ui/
  layout.go        // flexbox solver (Taffy/Morphorm-equivalent), measure→layout phases
  dirty.go         // dirty-flag propagation, deferred child sort, cached minSize
  interaction.go   // PreUpdate hit-test (reverse render order), event bubbling, focus nav
  atlas.go         // FontAtlas: glyph cache over render-core DynamicAtlas (INV-4)
  shape.go         // text shaping: (text, font, size) → positioned glyphs
  feature.go       // UiFeature: RenderFeature compositing UI last (INV-2), ZIndex resolve
```

## Type Definitions

```go
type ValKind uint8
const (ValPx ValKind = iota; ValPercent; ValAuto; ValVw; ValVh)
type Val struct { Kind ValKind; Value float32 }

type Display uint8;        const (DisplayFlex Display = iota; DisplayGrid; DisplayNone)
type PositionType uint8;   const (PositionRelative PositionType = iota; PositionAbsolute)
type FlexDirection uint8;  const (Row FlexDirection = iota; Column; RowReverse; ColumnReverse)
// JustifyContent, AlignItems, AlignSelf, AlignContent, FlexWrap, Overflow, Direction — iota enums

type UiRect struct { Left, Right, Top, Bottom Val }

type Style struct {
    Display        Display
    Position       PositionType
    Overflow       Overflow
    FlexDirection  FlexDirection
    FlexWrap       FlexWrap
    JustifyContent JustifyContent
    AlignItems     AlignItems
    AlignSelf      AlignSelf
    AlignContent   AlignContent
    Width, Height  Val
    MinWidth, MinHeight, MaxWidth, MaxHeight Val
    Margin, Padding, Border UiRect
    Gap            pkgmath.Vec2 // row gap, column gap
    FlexGrow       float32
    FlexShrink     float32
    FlexBasis      Val
    AspectRatio    *float32
}

type Node struct{} // base marker; pairs with Style + LayoutRect

// LayoutRect is the solver's output — logical pixels.
type LayoutRect struct { Position pkgmath.Vec2; Size pkgmath.Vec2 }

type Interaction uint8
const (InteractionNone Interaction = iota; InteractionHovered; InteractionPressed)

type MouseFilter uint8
const (MouseStop MouseFilter = iota; MousePass; MouseIgnore)

type ZIndex struct { Local int32; Global int32; UseGlobal bool }

type TextSection struct { Value string; Font asset.Handle[Font]; FontSize float32; Color pkgmath.Color }
type Text struct {
    Sections   []TextSection
    Alignment  TextAlignment
    Wrapping   TextWrapping
    LineHeight float32
}

type OffsetTransform struct {
    Translation pkgmath.Vec2 // pixel offset from layout
    Rotation    float32      // radians around pivot
    Scale       pkgmath.Vec2
    Pivot       pkgmath.Vec2 // 0.5,0.5 = center
}

type FocusNeighbor struct { Left, Right, Up, Down ecs.EntityID } // 0 ⇒ none
```

## Key Methods

```go
func (p UiPlugin) Build(app *app.App) // registers layoutSystem(PostUpdate), interactionSystem(PreUpdate), UiFeature

// Layout (INV-1): solve only dirty subtrees.
func layoutSystem(/* Query[Changed[Style], &LayoutRect, Children], Res[Viewport] */)
func (s *solver) solve(root ecs.EntityID) // measure → layout, writes LayoutRect

// Interaction (INV-3): PreUpdate hit-test + bubble.
func interactionSystem(/* Res[PointerState], Query[LayoutRect, MouseFilter, &Interaction] */)

// Font atlas (INV-4): cached glyph lookup.
func (a *FontAtlas) Glyph(font asset.Handle[Font], sizePx float32, r rune) (atlasEntry, bool)

// UiFeature (INV-2): composite UI last.
func (f *UiFeature) Draw(ctx *render.DrawContext, view *render.RenderView)
var _ render.RenderFeature = (*UiFeature)(nil)
```

## Performance Strategy

- **Dirty-subtree relayout (INV-1)**: only entities with `Changed[Style]` (or under a reparented root) are re-solved; an idle UI costs one cheap change-tick scan, zero solving.
- **Cached minimum size**: per-node intrinsic size is memoized and recomputed only when `notifyMinSizeChanged` flags it dirty; the flag propagates upward so ancestors reflow exactly once.
- **Deferred child sort**: container re-sort is batched to the layout phase via a `pending_sort` flag, avoiding O(n²) relayout during bulk spawn (L1 §4.15).
- **Glyph atlas reuse (INV-4)**: shaped glyphs hit the `FontAtlas` cache; only first-seen `(font,size,glyph)` triples rasterize. Quad meshes batch by atlas texture (reuses 2D batching).
- **OffsetTransform is render-only**: animation offsets/scale/rotation apply in the feature's `Draw`, never invalidating layout — a wobbling button triggers no relayout of siblings.

## Error Handling

| Condition | Behavior |
| :--- | :--- |
| `Style` with conflicting min/max (min > max) | solver clamps `max = min`; `slog.Debug` (no panic) |
| Missing/failed `Font` handle | falls back to the engine default font; `slog.Warn` once per handle |
| Atlas full and cannot grow | rebuild at next power-of-two size; if the largest glyph exceeds max texture size ⇒ `ErrGlyphTooLarge` |
| Reparent creating a hierarchy cycle | rejected by the hierarchy layer (`ChildOf` guard); layout never sees a cyclic tree |
| `FocusNeighbor` pointing at a despawned entity | treated as `none`; focus stays put; `slog.Debug` |

```go
type ErrGlyphTooLarge struct{ Rune rune; SizePx float32 }
```

## Testing Strategy

- **INV-1**: mutating one node's `Style` re-solves only its subtree (assert sibling `LayoutRect`s unchanged + a relayout counter); an idle frame solves nothing.
- **Flexbox correctness**: golden table of (Style tree → expected LayoutRects) for row/column, grow/shrink, justify/align, percent/auto sizing.
- **INV-3**: pointer over a stacked layout updates `Interaction` on the top-most `MouseStop` node only; `MousePass` propagates; `MouseIgnore` is skipped; events bubble parent-ward until stopped.
- **INV-4**: rendering the same string twice rasterizes glyphs once (atlas entry count stable on the second pass); distinct sizes produce distinct entries.
- **INV-2**: `UiFeature` draws after all scene/2D features (assert pass order); ZIndex overrides painter order deterministically.
- **OffsetTransform**: applying an offset changes draw output but not `LayoutRect` (no relayout triggered).
- **Determinism**: a fixed Style tree produces an identical batched draw topology hash across ≥20 runs (mirrors Phase 5 example pattern).

## 7. Drawbacks & Alternatives

- **Drawback**: no CSS cascade means more explicit `Style` data per node.
  **Alternative**: a cascading style resolver.
  **Decision**: L1 §2 mandates explicit-per-entity styling (ECS philosophy); cascade adds hidden coupling. Kept (open question deferred).
- **Drawback**: TTF shaping likely needs `golang.org/x/image/font` (an x/ dep).
  **Alternative**: bundle a stdlib-only bitmap font.
  **Decision**: x/image is the remote-stdlib candidate per CLAUDE.md priority order; gate behind an ADR. A stdlib bitmap fallback ships as the default font so the core stays dep-free until the ADR lands.
- **Drawback**: a single `UiFeature` pass limits per-camera UI customization.
  **Alternative**: per-camera UI features.
  **Decision**: `UiCameraConfig.ShowUI` covers multi-camera toggling; per-camera features are a future refinement. Kept.

## Canonical References

<!-- All paths verified on disk; validated by examples/ui (layout hash ×20, T-6T06)
     + pkg/ui 95.5% / internal/ui 89.8% cov. C29 P6 gate closed. Grid/wrap/AlignSelf,
     OffsetTransform, the ECS PreUpdate system, and a real TTF rasterizer
     (x/image/font ADR) are deferred refinements. -->

| Alias | Path | Purpose |
| :--- | :--- | :--- |
| [STYLE] | `pkg/ui/style.go` | `Style` component, `Val` tagged union, `UiRect`, Display/Flex/Justify/Align enums. |
| [NODE] | `pkg/ui/node.go` | `Node`, `LayoutRect` (+`Contains`), `ZIndex`, `BackgroundColor`/`BorderColor`. |
| [WIDGETS] | `pkg/ui/widgets.go` | `Text`/`TextSection`/`ImageNode`/`Font` + alignment/wrapping enums. |
| [INTERACTION] | `pkg/ui/interaction.go` | `Interaction`, `MouseFilter`, `Focused`, `TabIndex`, `FocusNeighbor`. |
| [LAYOUT] | `internal/ui/layout.go` | Single-line flexbox `Solve`/`SolveIfDirty` dirty-gate (INV-1). |
| [ATLAS] | `internal/ui/atlas.go` | `FontAtlas` glyph cache over render-core `DynamicAtlas` (INV-4). |
| [HITTEST] | `internal/ui/interaction.go` | `HitTest` (reverse render order, MouseFilter) + `InteractionFor` (INV-3). |
| [FEATURE] | `internal/ui/feature.go` | `UiFeature` (RenderFeature) compositing last + `SortByZ` (INV-2). |
| [EXAMPLE] | `examples/ui/main.go` | Toolbar layout, dirty-gate, layout hash-stable ×20 (T-6T06). |
| [TEST] | `pkg/ui/ui_test.go`, `internal/ui/ui_test.go` | Val resolve, flexbox golden rects, dirty-gate, atlas cache, hit-test, z-sort. |

## Document History

| Version | Date | Description |
| :--- | :--- | :--- |
| 0.1.0 | 2026-05-30 | Initial L2 draft — Go translation of l1-ui-system v0.2.0. `Style` component + `Val` tagged union, flexbox solver with dirty-subtree relayout via `Changed[Style]` (INV-1), `Interaction` hit-test in `PreUpdate` with MouseFilter + bubbling + focus neighbors (INV-3), `FontAtlas` glyph cache over render-core `DynamicAtlas` (INV-4), `UiFeature` compositing UI last via `RenderFeature` + ZIndex resolve (INV-2), render-only `OffsetTransform`, deferred child sort + cached minSize. Text shaping dep (x/image/font) flagged for ADR. Authored ahead of Phase 6 (`/magic.spec`). Draft — L1 parent Draft + no implementation yet. |
