package query

import (
	"reflect"
	"unsafe"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
)

// componentIDFor registers (if needed) and returns the [component.ID] for T
// in the world's registry. Used by query constructors to translate type
// parameters into IDs at build time.
func componentIDFor[T any](w *world.World) component.ID {
	t := reflect.TypeFor[T]()
	return w.Components().RegisterByType(t)
}

// fetchComponent returns the unsafe pointer to component id on entity e at
// the given row of arch. It chooses Table or SparseSet storage based on the
// component's [component.Info]. Returns nil for zero-size (tag) components
// — callers cast to *T regardless; reading through a nil *T of a zero-size
// type is fine because the type carries no fields.
func fetchComponent(w *world.World, arch *world.Archetype, e entity.Entity, row int, id component.ID) unsafe.Pointer {
	info := w.Components().Info(id)
	if info == nil {
		return nil
	}
	if info.Storage == component.StorageSparseSet {
		ss, ok := w.SparseSet(id)
		if !ok {
			return nil
		}
		ptr, _ := ss.Get(e)
		return ptr
	}
	tbl := arch.Table()
	if tbl == nil {
		return nil
	}
	ptr, _ := tbl.CellPtrByID(id, row)
	return ptr
}
