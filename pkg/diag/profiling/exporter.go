package profiling

import "errors"

// ProfileExporter is a pluggable span sink. The instrumentation API depends
// only on this interface, so adding a backend (Chrome, pprof, or a future
// Tracy CGo bridge) never touches BeginSpan/End (INV-5). All methods must be
// safe to call from the goroutine that ends a span; exporters that buffer are
// responsible for their own synchronisation.
type ProfileExporter interface {
	// Init prepares the exporter (opens files, interns strings). Called once.
	Init() error
	// EmitSpan records one completed span. The Span is passed by value; the
	// exporter must not retain the pointer-free copy beyond what it buffers.
	EmitSpan(span Span)
	// EmitFrameMark records a frame boundary with an incrementing frame number.
	EmitFrameMark(frame uint64)
	// Flush writes any buffered data without closing the exporter.
	Flush() error
	// Shutdown flushes and releases resources. Called once at teardown.
	Shutdown() error
}

// NopExporter discards everything. It is the default sink when profiling is
// dormant (ExporterNone) and a safe zero value.
type NopExporter struct{}

// Init does nothing.
func (NopExporter) Init() error { return nil }

// EmitSpan discards the span.
func (NopExporter) EmitSpan(Span) {}

// EmitFrameMark discards the frame mark.
func (NopExporter) EmitFrameMark(uint64) {}

// Flush does nothing.
func (NopExporter) Flush() error { return nil }

// Shutdown does nothing.
func (NopExporter) Shutdown() error { return nil }

// MultiExporter fans every span and frame mark out to a set of exporters,
// enabling e.g. pprof + Chrome simultaneously. Init/Flush/Shutdown errors are
// joined so a single failing backend does not mask the others.
type MultiExporter struct {
	exporters []ProfileExporter
}

// NewMultiExporter returns a MultiExporter wrapping the given backends. nil
// entries are dropped.
func NewMultiExporter(exporters ...ProfileExporter) *MultiExporter {
	kept := make([]ProfileExporter, 0, len(exporters))
	for _, e := range exporters {
		if e != nil {
			kept = append(kept, e)
		}
	}
	return &MultiExporter{exporters: kept}
}

// Len reports how many backends the MultiExporter dispatches to.
func (m *MultiExporter) Len() int { return len(m.exporters) }

// Init initialises every backend, joining any errors.
func (m *MultiExporter) Init() error {
	var errs []error
	for _, e := range m.exporters {
		if err := e.Init(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// EmitSpan forwards the span to every backend.
func (m *MultiExporter) EmitSpan(span Span) {
	for _, e := range m.exporters {
		e.EmitSpan(span)
	}
}

// EmitFrameMark forwards the frame mark to every backend.
func (m *MultiExporter) EmitFrameMark(frame uint64) {
	for _, e := range m.exporters {
		e.EmitFrameMark(frame)
	}
}

// Flush flushes every backend, joining any errors.
func (m *MultiExporter) Flush() error {
	var errs []error
	for _, e := range m.exporters {
		if err := e.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Shutdown shuts down every backend, joining any errors.
func (m *MultiExporter) Shutdown() error {
	var errs []error
	for _, e := range m.exporters {
		if err := e.Shutdown(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
