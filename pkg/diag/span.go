//go:build profiling

package diag

import (
	"log/slog"
	"time"
)

// Span records the wall-clock duration of a code region. Compiled only under
// -tags profiling (INV-4); the default build links the no-op variant. End emits
// the timing at Debug level — a Tracy/Chrome-trace exporter can replace this
// sink without changing call sites.
type Span struct {
	name  string
	start time.Time
}

// StartSpan begins timing a named region.
func StartSpan(name string) Span {
	return Span{name: name, start: time.Now()}
}

// End records the elapsed time for the span.
func (s Span) End() {
	slog.Debug("profiling.span", "name", s.name, "dur_us", time.Since(s.start).Microseconds())
}
