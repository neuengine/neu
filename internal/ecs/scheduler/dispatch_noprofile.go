//go:build !profiling

package scheduler

import "github.com/neuengine/neu/internal/ecs/world"

// dispatchRun invokes a system. Without the profiling build tag it inlines to a
// direct call, so the default build carries zero instrumentation overhead.
func dispatchRun(sys System, w *world.World) {
	sys.Run(w)
}
