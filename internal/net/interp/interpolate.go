package interp

import (
	"encoding/json"
	"maps"
	"reflect"
	"time"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/world"
	neumath "github.com/neuengine/neu/pkg/math"
)

// DisplayState is a render-only component that holds the interpolated component
// values for one entity. It is written exclusively by InterpolateSystem and must
// never be modified by simulation systems (INV-1).
type DisplayState struct {
	Components map[string]json.RawMessage // typeName → interpolated JSON
}

// lerpFunc blends two JSON-encoded component values at factor t.
// t is in [0, 1] for normal interpolation; values slightly outside [0,1] are
// used for bounded extrapolation.
type lerpFunc func(a, b json.RawMessage, t float32, reg *component.Registry) json.RawMessage

// DefaultMaxExtrapolation caps the extrapolation factor beyond the latest snapshot.
const DefaultMaxExtrapolation float32 = 0.5

// lerpTable maps component Go types to their interpolation functions.
// Types not present here fall back to snap (holding a).
var lerpTable = map[reflect.Type]lerpFunc{
	reflect.TypeFor[neumath.Vec2](): makeLerper(func(a, b neumath.Vec2, t float32) neumath.Vec2 {
		return a.Lerp(b, t)
	}),
	reflect.TypeFor[neumath.Vec3](): makeLerper(func(a, b neumath.Vec3, t float32) neumath.Vec3 {
		return a.Lerp(b, t)
	}),
	reflect.TypeFor[neumath.Vec4](): makeLerper(func(a, b neumath.Vec4, t float32) neumath.Vec4 {
		return a.Lerp(b, t)
	}),
	reflect.TypeFor[neumath.Quat](): makeLerper(func(a, b neumath.Quat, t float32) neumath.Quat {
		// Slerp is undefined outside [0,1]; clamp for rotations only.
		ct := t
		if ct < 0 {
			ct = 0
		} else if ct > 1 {
			ct = 1
		}
		return a.Slerp(b, ct)
	}),
	reflect.TypeFor[float32](): func(aJSON, bJSON json.RawMessage, t float32, _ *component.Registry) json.RawMessage {
		var av, bv float32
		if err := json.Unmarshal(aJSON, &av); err != nil {
			return aJSON
		}
		if err := json.Unmarshal(bJSON, &bv); err != nil {
			return aJSON
		}
		result := av + (bv-av)*t
		out, err := json.Marshal(result)
		if err != nil {
			return aJSON
		}
		return out
	},
	reflect.TypeFor[float64](): func(aJSON, bJSON json.RawMessage, t float32, _ *component.Registry) json.RawMessage {
		var av, bv float64
		if err := json.Unmarshal(aJSON, &av); err != nil {
			return aJSON
		}
		if err := json.Unmarshal(bJSON, &bv); err != nil {
			return aJSON
		}
		result := av + (bv-av)*float64(t)
		out, err := json.Marshal(result)
		if err != nil {
			return aJSON
		}
		return out
	},
}

// makeLerper generates a lerpFunc for a concrete math type T.
func makeLerper[T any](blend func(a, b T, t float32) T) lerpFunc {
	return func(aJSON, bJSON json.RawMessage, t float32, _ *component.Registry) json.RawMessage {
		var av, bv T
		if err := json.Unmarshal(aJSON, &av); err != nil {
			return aJSON
		}
		if err := json.Unmarshal(bJSON, &bv); err != nil {
			return aJSON
		}
		out, err := json.Marshal(blend(av, bv, t))
		if err != nil {
			return aJSON
		}
		return out
	}
}

// RegisterLerper installs a custom lerp function for a component type T.
// Call before any InterpolateSystem.Run to override the built-in table.
func RegisterLerper[T any](fn func(a, b T, t float32) T) {
	lerpTable[reflect.TypeFor[T]()] = makeLerper(fn)
}

// lerpComponent blends aJSON and bJSON at factor t using the registered lerp
// function for the named type. Returns aJSON (snap) if the type is unknown or
// unmarshalling fails.
func lerpComponent(typeName string, aJSON, bJSON json.RawMessage, t float32, reg *component.Registry) json.RawMessage {
	id, ok := reg.LookupByName(typeName)
	if !ok {
		return aJSON // unknown type — snap
	}
	info := reg.Info(id)
	fn, ok := lerpTable[info.Type]
	if !ok {
		return aJSON // no lerper — snap
	}
	return fn(aJSON, bJSON, t, reg)
}

// InterpolateSystem reads from a SnapshotBuffer and writes interpolated entity
// state into DisplayState components. It runs in PostUpdate, after replication
// receive commands have been applied. It never modifies authoritative components
// (INV-1).
type InterpolateSystem struct {
	buf              *SnapshotBuffer
	renderDelay      time.Duration
	maxExtrapolation float32
	extrapolate      bool
	// lookupClient translates a server EntityID to a client EntityID.
	// May be nil when server and client IDs are identical.
	lookupClient func(entity.EntityID) (entity.EntityID, bool)
}

// NewInterpolateSystem creates an InterpolateSystem backed by buf.
// renderDelay is how far behind the latest snapshot the render clock lags.
// maxExtrapolation caps the extrapolation factor (DefaultMaxExtrapolation = 0.5).
// lookupClient may be nil (server ID used directly as client ID).
func NewInterpolateSystem(
	buf *SnapshotBuffer,
	renderDelay time.Duration,
	extrapolate bool,
	maxExtrapolation float32,
	lookupClient func(entity.EntityID) (entity.EntityID, bool),
) *InterpolateSystem {
	if maxExtrapolation <= 0 {
		maxExtrapolation = DefaultMaxExtrapolation
	}
	return &InterpolateSystem{
		buf:              buf,
		renderDelay:      renderDelay,
		maxExtrapolation: maxExtrapolation,
		extrapolate:      extrapolate,
		lookupClient:     lookupClient,
	}
}

// SetRenderDelay updates the render-clock lag. Call from AdaptiveDelay each frame.
func (sys *InterpolateSystem) SetRenderDelay(d time.Duration) { sys.renderDelay = d }

// Run is the ECS system entry point; called once per frame in PostUpdate.
func (sys *InterpolateSystem) Run(w *world.World) {
	reg := w.Components()

	if sys.buf.Len() < 2 {
		// INV-2 warm-up: display the latest snapshot directly (no interpolation).
		if e, ok := sys.buf.Latest(); ok {
			sys.applySnapshot(w, e)
		}
		return
	}

	latest, _ := sys.buf.Latest()
	renderTime := latest.Timestamp - sys.renderDelay

	prev, next, ok := sys.buf.Bracket(renderTime)
	if !ok {
		if sys.extrapolate && renderTime > latest.Timestamp && sys.buf.Len() >= 2 {
			sys.applyExtrapolation(w, reg, renderTime)
			return
		}
		// No bracket and no extrapolation — snap to latest (INV-4).
		sys.applySnapshot(w, latest)
		return
	}

	// Compute t ∈ [0, 1] — clamp per INV-4.
	t := computeT(renderTime, prev.Timestamp, next.Timestamp)
	sys.applyBlend(w, reg, prev, next, t)
}

// applySnapshot writes the snapshot entry's component values directly as DisplayState
// (warm-up / gap fallback, INV-2).
func (sys *InterpolateSystem) applySnapshot(w *world.World, e SnapshotEntry) {
	for serverID, state := range e.Entities {
		clientE, ok := sys.resolveClient(serverID)
		if !ok {
			continue
		}
		if _, has := w.ArchetypeOf(clientE); !has {
			continue
		}
		comps := make(map[string]json.RawMessage, len(state))
		maps.Copy(comps, state)
		_ = w.Insert(clientE, component.Data{Value: DisplayState{Components: comps}})
	}
}

// applyBlend interpolates between prev and next at factor t, writing DisplayState.
func (sys *InterpolateSystem) applyBlend(w *world.World, reg *component.Registry, prev, next SnapshotEntry, t float32) {
	for serverID, nextState := range next.Entities {
		prevState, hasPrev := prev.Entities[serverID]
		clientE, ok := sys.resolveClient(serverID)
		if !ok {
			continue
		}
		if _, has := w.ArchetypeOf(clientE); !has {
			continue
		}

		comps := make(map[string]json.RawMessage, len(nextState))
		for typeName, bJSON := range nextState {
			if !hasPrev {
				comps[typeName] = bJSON
				continue
			}
			aJSON, inPrev := prevState[typeName]
			if !inPrev {
				comps[typeName] = bJSON
				continue
			}
			comps[typeName] = lerpComponent(typeName, aJSON, bJSON, t, reg)
		}
		_ = w.Insert(clientE, component.Data{Value: DisplayState{Components: comps}})
	}
}

// applyExtrapolation extends the latest two entries beyond the newest snapshot (INV-4).
func (sys *InterpolateSystem) applyExtrapolation(w *world.World, reg *component.Registry, renderTime time.Duration) {
	n := sys.buf.Len()
	ring := sys.buf.ring
	a, b := ring[n-2], ring[n-1]

	dt := b.Timestamp - a.Timestamp
	var extraFactor float32
	if dt > 0 {
		extraFactor = float32(float64(renderTime-b.Timestamp) / float64(dt))
	}
	if extraFactor > sys.maxExtrapolation {
		extraFactor = sys.maxExtrapolation
	}
	if extraFactor < 0 {
		extraFactor = 0
	}
	// t=1+extraFactor: lerp beyond b using the a→b direction.
	sys.applyBlend(w, reg, a, b, 1+extraFactor)
}

// resolveClient converts a server EntityID to the corresponding client Entity.
func (sys *InterpolateSystem) resolveClient(serverID entity.EntityID) (entity.Entity, bool) {
	if sys.lookupClient == nil {
		return entity.FromID(serverID), true
	}
	clientID, ok := sys.lookupClient(serverID)
	if !ok {
		return entity.Entity{}, false
	}
	return entity.FromID(clientID), true
}

// computeT computes the interpolation factor t ∈ [0, 1] for renderTime between
// prevTS and nextTS (INV-4: clamped).
func computeT(renderTime, prevTS, nextTS time.Duration) float32 {
	dt := nextTS - prevTS
	if dt <= 0 {
		return 0
	}
	t := float32(float64(renderTime-prevTS) / float64(dt))
	if t < 0 {
		return 0
	}
	if t > 1 {
		return 1
	}
	return t
}
