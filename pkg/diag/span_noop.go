//go:build !profiling

package diag

// Span is a profiling span. In a non-profiling build it is an empty struct and
// StartSpan/End are inlinable no-ops, so instrumentation costs nothing in
// release binaries (INV-4: profiling spans are compile-time removable).
type Span struct{}

// StartSpan returns a zero Span. No-op without the profiling build tag.
func StartSpan(string) Span { return Span{} }

// End is a no-op without the profiling build tag.
func (Span) End() {}
