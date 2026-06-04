package scene

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
)

// Hydrate reconstructs a DynamicScene from a portable SerializedScene, resolving
// each component's interned type name through the TypeRegistry and rebuilding its
// field values via a JSON round-trip (the same stdlib coercion the World-extract
// path relies on — number widening, nested structs, slices). It completes the
// .scene.json vertical: a loaded SerializedScene → a spawnable DynamicScene.
//
// Entity identity is positional: entity i is assigned EntityID(i+1), so any
// inter-entity EntityID reference stored as that index is remapped to the freshly
// allocated entity by SceneSpawner.Spawn's pass-2 walk.
//
// Failures are per-component and collected, not fatal (forward-compat, INV like
// the definition instantiator): an unknown type or an unconstructible value is
// skipped with an error so a partially-decodable scene still spawns what it can.
func Hydrate(sc *SerializedScene, reg *typereg.TypeRegistry) (DynamicScene, []error) {
	if sc == nil || reg == nil {
		return DynamicScene{}, nil
	}
	var errs []error
	ents := make([]DynamicEntity, 0, len(sc.Entities))
	for ei := range sc.Entities {
		de := DynamicEntity{ID: entity.EntityID(ei + 1)}
		for ci := range sc.Entities[ei].Components {
			typeName := sc.ComponentType(ei, ci)
			treg := reg.ResolveByName(typeName)
			if treg == nil {
				errs = append(errs, fmt.Errorf("scene: unknown component type %q", typeName))
				continue
			}
			props := propMap(sc, ei, ci)
			val, err := constructComponent(treg.Type, props)
			if err != nil {
				errs = append(errs, fmt.Errorf("scene: component %q: %w", typeName, err))
				continue
			}
			de.Components = append(de.Components, ReflectedComponent{Reg: treg, Value: val})
		}
		ents = append(ents, de)
	}
	return DynamicScene{entities: ents}, errs
}

// SpawnSerialized hydrates sc through the TypeRegistry and spawns the result into
// w in one step, returning the instance ID plus any per-component hydration
// errors (a non-zero ID with errors means a partial scene was spawned).
func (s *SceneSpawner) SpawnSerialized(w WorldWriter, sc *SerializedScene, reg *typereg.TypeRegistry) (InstanceID, []error) {
	ds, errs := Hydrate(sc, reg)
	return s.Spawn(w, &ds), errs
}

// propMap collects a component's (fieldName → value) pairs from the interned
// Names/Variants tables.
func propMap(sc *SerializedScene, entityIdx, compIdx int) map[string]any {
	comp := sc.Entities[entityIdx].Components[compIdx]
	props := make(map[string]any, len(comp.Props))
	for pi := range comp.Props {
		nameIdx := comp.Props[pi][0]
		if nameIdx < 0 || nameIdx >= len(sc.Names) {
			continue
		}
		props[sc.Names[nameIdx]] = sc.PropertyValue(entityIdx, compIdx, pi)
	}
	return props
}

// constructComponent builds a value of type t from the field map via a JSON
// round-trip (reusing the stdlib decoder's coercion) and returns it as a
// reflect.Value ready for a ReflectedComponent.
func constructComponent(t reflect.Type, props map[string]any) (reflect.Value, error) {
	b, err := json.Marshal(props)
	if err != nil {
		return reflect.Value{}, err
	}
	ptr := reflect.New(t)
	if err := json.Unmarshal(b, ptr.Interface()); err != nil {
		return reflect.Value{}, err
	}
	return ptr.Elem(), nil
}
