//go:build debug

package errs

import "runtime"

// captureTrace records the call stack at error-construction time. Compiled only
// under -tags debug (INV: trace context in debug builds). The first frames
// (this function + New/Wrap) are skipped so the trace starts at the caller.
func captureTrace() []uintptr {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(3, pcs)
	return pcs[:n]
}
