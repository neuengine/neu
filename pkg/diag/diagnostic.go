// Package diag provides runtime diagnostics: a DiagnosticsStore of named
// metrics over fixed-capacity ring buffers (0-alloc Push, deterministic
// sample-count averages), immediate-mode gizmos drained by a render feature,
// a per-module slog filter, and build-tag-gated profiling spans. All collection
// is zero-cost when no reader is registered (INV-1).
//
// Bootstrap: l2-diagnostic-system-go Draft (Phase 6 Track C, C29 gate open).
package diag

import (
	"math"
	"time"
)

// DiagnosticPath is a slash-delimited metric identifier, e.g. "engine/fps".
type DiagnosticPath string

// DiagnosticEntry is one recorded sample. Timestamp is informational; averages
// use sample count (not wall-clock), so they are deterministic under replay.
type DiagnosticEntry struct {
	Value     float64
	Timestamp time.Duration
}

// RingBuffer is a fixed-capacity circular buffer. After construction every Push
// overwrites the oldest slot in place — no growth, no per-sample allocation.
type RingBuffer[T any] struct {
	buf  []T
	head int // index of the next write
	size int // number of valid entries (<= cap)
}

// NewRingBuffer returns a ring buffer of the given capacity (clamped to >= 1).
func NewRingBuffer[T any](capacity int) RingBuffer[T] {
	if capacity < 1 {
		capacity = 1
	}
	return RingBuffer[T]{buf: make([]T, capacity)}
}

// Push records v, overwriting the oldest entry once the buffer is full.
func (r *RingBuffer[T]) Push(v T) {
	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
	if r.size < len(r.buf) {
		r.size++
	}
}

// Len returns the number of valid entries.
func (r *RingBuffer[T]) Len() int { return r.size }

// Cap returns the buffer capacity.
func (r *RingBuffer[T]) Cap() int { return len(r.buf) }

// ForEach calls fn for each entry oldest→newest without allocating.
func (r *RingBuffer[T]) ForEach(fn func(T)) {
	if r.size == 0 {
		return
	}
	start := (r.head - r.size + len(r.buf)) % len(r.buf)
	for i := range r.size {
		fn(r.buf[(start+i)%len(r.buf)])
	}
}

// Latest returns the most recently pushed entry, or (zero, false) if empty.
func (r *RingBuffer[T]) Latest() (T, bool) {
	var zero T
	if r.size == 0 {
		return zero, false
	}
	idx := (r.head - 1 + len(r.buf)) % len(r.buf)
	return r.buf[idx], true
}

// DefaultHistory is the ring-buffer capacity for a diagnostic when unspecified.
const DefaultHistory = 120

// smoothingAlpha weights the newest sample in the exponential moving average.
const smoothingAlpha = 0.2

// Diagnostic is a named measurement with a rolling history (L1 §4.2).
type Diagnostic struct {
	Path     DiagnosticPath
	Suffix   string // unit label: "fps", "ms", "count"
	history  RingBuffer[DiagnosticEntry]
	enabled  bool
	readers  int
	smoothed float64
	smoothOK bool
}

// NewDiagnostic creates an enabled diagnostic with a history of maxHistory
// samples (DefaultHistory when <= 0).
func NewDiagnostic(path DiagnosticPath, suffix string, maxHistory int) *Diagnostic {
	if maxHistory <= 0 {
		maxHistory = DefaultHistory
	}
	return &Diagnostic{
		Path:    path,
		Suffix:  suffix,
		history: NewRingBuffer[DiagnosticEntry](maxHistory),
		enabled: true,
	}
}

// record appends a sample and updates the smoothed average.
func (d *Diagnostic) record(value float64, ts time.Duration) {
	d.history.Push(DiagnosticEntry{Value: value, Timestamp: ts})
	if d.smoothOK {
		d.smoothed = smoothingAlpha*value + (1-smoothingAlpha)*d.smoothed
	} else {
		d.smoothed = value
		d.smoothOK = true
	}
}

// Average returns the arithmetic mean over the history buffer (0 if empty).
func (d *Diagnostic) Average() float64 {
	n := d.history.Len()
	if n == 0 {
		return 0
	}
	var sum float64
	d.history.ForEach(func(e DiagnosticEntry) { sum += e.Value })
	return sum / float64(n)
}

// SmoothedAverage returns the exponentially weighted moving average.
func (d *Diagnostic) SmoothedAverage() float64 { return d.smoothed }

// Min returns the smallest value in the history buffer (0 if empty).
func (d *Diagnostic) Min() float64 {
	if d.history.Len() == 0 {
		return 0
	}
	m := math.Inf(1)
	d.history.ForEach(func(e DiagnosticEntry) {
		if e.Value < m {
			m = e.Value
		}
	})
	return m
}

// Max returns the largest value in the history buffer (0 if empty).
func (d *Diagnostic) Max() float64 {
	if d.history.Len() == 0 {
		return 0
	}
	m := math.Inf(-1)
	d.history.ForEach(func(e DiagnosticEntry) {
		if e.Value > m {
			m = e.Value
		}
	})
	return m
}

// Latest returns the most recent entry, or (zero, false) if empty.
func (d *Diagnostic) Latest() (DiagnosticEntry, bool) { return d.history.Latest() }

// Len reports how many samples are currently buffered.
func (d *Diagnostic) Len() int { return d.history.Len() }
