package render

import "github.com/neuengine/neu/internal/ecs/world"

// ExtractFn copies relevant state from the main World into the render World
// (l1-render-core §4.1). It MUST copy, never retain pointers into main —
// the render world is an isolated per-frame snapshot (INV-4, l1-render-core
// §2 "no shared mutable state"). Signature mirrors app.ExtractFn so a render
// plugin can adapt RunFrame into an app.SubApp without a type bridge.
type ExtractFn func(main, render *world.World)

// extractRegistry is the ordered set of extract functions a [RenderSubApp]
// runs each frame, in registration order (deterministic).
type extractRegistry struct {
	fns []ExtractFn
}

func (r *extractRegistry) add(fn ExtractFn) {
	if fn != nil {
		r.fns = append(r.fns, fn)
	}
}

// run invokes every registered extract fn in order. The caller guarantees
// single-invocation-per-frame (INV-4) — this method does not self-guard.
func (r *extractRegistry) run(main, render *world.World) {
	for _, fn := range r.fns {
		fn(main, render)
	}
}
