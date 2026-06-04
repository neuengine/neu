//go:build profiling

package profiling

import "time"

// PprofStat is the aggregated timing for one span name.
type PprofStat struct {
	Count   uint64
	TotalNs int64
}

// Mean returns the average span duration for the name.
func (s PprofStat) Mean() time.Duration {
	if s.Count == 0 {
		return 0
	}
	return time.Duration(s.TotalNs / int64(s.Count))
}

// PprofExporter aggregates span timings by name (count + total duration). It is
// pure Go (no CGo dependency, C-003) and complements the standard Go pprof
// endpoints the DiagnosticsPlugin already exposes (CPU/heap/goroutine profiles
// over net/http/pprof).
//
// Scope note: true runtime/pprof *sample labels* must wrap execution via
// pprof.Do at the dispatch site — they cannot be attached to an already-ended
// span. This exporter therefore surfaces aggregate span timings (a "where did
// frame time go by system" view) rather than per-sample labels; wiring
// pprof.Labels into the scheduler dispatch is a documented follow-up.
type PprofExporter struct {
	stats map[string]PprofStat
}

// NewPprofExporter returns an empty aggregator.
func NewPprofExporter() *PprofExporter {
	return &PprofExporter{stats: make(map[string]PprofStat)}
}

// Init is a no-op.
func (p *PprofExporter) Init() error { return nil }

// EmitSpan folds the span's duration into the per-name aggregate.
func (p *PprofExporter) EmitSpan(s Span) {
	st := p.stats[s.name]
	st.Count++
	st.TotalNs += s.endNs - s.startNs
	p.stats[s.name] = st
}

// EmitFrameMark is a no-op (pprof has no frame concept).
func (p *PprofExporter) EmitFrameMark(uint64) {}

// Flush is a no-op (stats live in memory until queried).
func (p *PprofExporter) Flush() error { return nil }

// Shutdown is a no-op.
func (p *PprofExporter) Shutdown() error { return nil }

// Stat returns the aggregate for a span name and whether it was recorded.
func (p *PprofExporter) Stat(name string) (PprofStat, bool) {
	st, ok := p.stats[name]
	return st, ok
}
