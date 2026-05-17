// Package editor exposes the engine-side boundary interfaces for the external
// editor process. All content is interface declarations and data types — no
// function bodies, no internal/ imports (INV-3 of l2-multi-repo-architecture-go).
//
// The concrete editor implementation lives in the external editor repository
// and depends only on this package and pkg/math.
package editor

import neu "github.com/neuengine/neu/pkg/math"

// Entity is a stable handle to an ECS entity. Value is opaque to the editor.
// In a future pkg/ecs package this will become a type alias.
type Entity uint64

// TypeID identifies a registered component or resource type by its reflection index.
type TypeID uint32

// DynamicObject is an opaque container for a component value whose concrete
// Go type is resolved at runtime. The editor receives it read-only.
type DynamicObject interface {
	// TypeID returns the registered component type identifier.
	TypeID() TypeID
}

// CommandBuffer is a handle for deferred ECS mutations issued through the
// editor boundary. The engine resolves the concrete type at runtime; this
// placeholder keeps pkg/editor free of internal/ imports (INV-3/INV-6).
type CommandBuffer struct{}

// EditorPlugin is the single entry point registered by the external editor.
// The engine invokes Build only during LEVEL_EDITOR init; absent in headless
// and production builds (INV-5).
type EditorPlugin interface {
	Build(api EditorInterface)
}

// EditorInterface is the capability handle passed to EditorPlugin.Build.
// Exposes only deferred mutation and read-only access — no *World (INV-6).
type EditorInterface interface {
	RegisterInspectorPlugin(InspectorPlugin)
	RegisterGizmoPlugin(GizmoPlugin)
	RegisterDefinitionPlugin(DefinitionEditorPlugin)
	Commands() *CommandBuffer
}

// InspectorPlugin renders a property panel for a specific component type.
type InspectorPlugin interface {
	Handles(componentTypeID TypeID) bool
	Render(entity Entity, component DynamicObject) PropertyList
}

// GizmoPlugin draws 3D editor overlays for a specific component type.
type GizmoPlugin interface {
	Handles(componentTypeID TypeID) bool
	Draw(entity Entity, component DynamicObject, gizmos GizmoWriter)
	// Interact returns (_, false) when the gizmo was not hit (L1 Option→comma-ok).
	Interact(entity Entity, component DynamicObject, ray neu.Ray3D) (GizmoHit, bool)
}

// DefinitionEditorPlugin handles editing of a specific definition node type.
type DefinitionEditorPlugin interface {
	Handles(defType DefinitionType) bool
	Edit(node DefinitionNode)
	GetInspectorProperties(node DefinitionNode) []EditorProperty
}
