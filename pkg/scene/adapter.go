package scene

import (
	"fmt"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
)

// WorldAdapter adapts a *world.World to the WorldReader and WorldWriter
// interfaces used by DynamicSceneBuilder and SceneSpawner.
type WorldAdapter struct {
	w    *world.World
	comp *component.Registry
}

// NewWorldAdapter wraps w for use with the scene system.
func NewWorldAdapter(w *world.World) *WorldAdapter {
	return &WorldAdapter{w: w, comp: w.Components()}
}

// World returns the underlying World.
func (a *WorldAdapter) World() *world.World { return a.w }

// EachArchetype implements WorldReader.
func (a *WorldAdapter) EachArchetype(fn func(arch ArchetypeView) bool) {
	a.w.Archetypes().Each(func(arch *world.Archetype) bool {
		return fn(&archetypeAdapter{arch: arch, comp: a.comp})
	})
}

// SpawnEmpty implements WorldWriter.
func (a *WorldAdapter) SpawnEmpty() entity.Entity {
	return a.w.SpawnEmpty()
}

// InsertComponent implements WorldWriter. ptrValue must be a pointer to a
// registered component type.
func (a *WorldAdapter) InsertComponent(e entity.Entity, ptrValue any) error {
	rv := reflect.ValueOf(ptrValue)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	t := rv.Type()
	id, ok := a.comp.Lookup(t)
	if !ok {
		return fmt.Errorf("scene: component type %v not registered in component registry", t)
	}
	return a.w.Insert(e, component.Data{Value: rv.Interface(), ID: id})
}

// ─── archetypeAdapter ────────────────────────────────────────────────────────

type archetypeAdapter struct {
	arch *world.Archetype
	comp *component.Registry
}

func (a *archetypeAdapter) Entities() []entity.Entity {
	return a.arch.Entities()
}

func (a *archetypeAdapter) ComponentValues(row int) map[string]any {
	tbl := a.arch.Table()
	if tbl == nil {
		return nil
	}
	rawValues := tbl.RowValues(row)
	out := make(map[string]any, len(rawValues))
	for id, val := range rawValues {
		info := a.comp.Info(id)
		if info.Type == nil {
			continue
		}
		out[info.Name] = val
	}
	return out
}
