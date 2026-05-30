//go:build !debug

package errs

// captureTrace is a no-op in release builds: the runtime.Callers machinery is
// excluded entirely, so error construction pays zero cost for traces (INV:
// trace context is captured only in debug builds).
func captureTrace() []uintptr { return nil }
