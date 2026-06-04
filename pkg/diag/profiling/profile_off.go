//go:build !profiling

package profiling

import "context"

// noopSpan is the single shared span returned when profiling is compiled out.
// End/Annotate on it are no-ops, so it is safe to share across goroutines.
var noopSpan = new(Span)

// BeginSpan is a no-op without the profiling build tag: it returns the context
// unchanged and a shared sentinel span, inlining to nothing in release builds
// (INV-1).
func BeginSpan(ctx context.Context, _ string, _ SpanCategory) (context.Context, *Span) {
	return ctx, noopSpan
}

// End is a no-op without the profiling build tag.
func (*Span) End() {}

// Annotate is a no-op without the profiling build tag.
func (*Span) Annotate(string, any) {}

// MarkFrame is a no-op without the profiling build tag.
func MarkFrame() {}
