//go:build profiling

package scheduler

import (
	"context"

	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/diag/profiling"
)

// dispatchRun wraps each system execution in a profiling span named
// "System:{name}" (CategorySystem) under the profiling build tag. The System
// interface carries no context.Context, so the span is a frame-local root
// (grouped by frame via MarkFrame); deeper nesting is opt-in by systems that
// create their own span context. defer ensures the span closes even if the
// system panics (recovered upstream in runSystemSafe).
func dispatchRun(sys System, w *world.World) {
	_, span := profiling.BeginSpan(context.Background(), "System:"+sys.Name(), profiling.CategorySystem)
	defer span.End()
	sys.Run(w)
}
