package view

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/world"
)

// TagOf resolves the [component.ID] for type T against the world's component
// registry, registering T on first use. The returned ID is stable for the
// lifetime of the World.
//
// Use this helper to avoid scattering `reflect.TypeFor[T]()` and
// `Components().Lookup` plumbing across system code.
func TagOf[T any](w *world.World) component.ID {
	return w.Components().RegisterByType(reflect.TypeFor[T]())
}

// MaskOf returns a [query.Mask] with only the bit for type T set.
func MaskOf[T any](w *world.World) query.Mask {
	return query.NewMask(TagOf[T](w))
}

// MaskOf2 returns a mask with the bits for both T1 and T2 set.
func MaskOf2[T1, T2 any](w *world.World) query.Mask {
	return query.NewMask(TagOf[T1](w), TagOf[T2](w))
}

// MaskOf3 returns a mask with the bits for T1, T2, and T3 set.
func MaskOf3[T1, T2, T3 any](w *world.World) query.Mask {
	return query.NewMask(TagOf[T1](w), TagOf[T2](w), TagOf[T3](w))
}

// MaskOfIDs builds a [query.Mask] directly from a slice of pre-resolved IDs.
// Useful when the caller already holds the IDs (e.g. via [TagOf] reuse).
func MaskOfIDs(ids ...component.ID) query.Mask {
	return query.NewMask(ids...)
}
