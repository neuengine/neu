package diag

import (
	"log/slog"
	"sync"
	"time"
)

// DiagnosticsStore holds all registered diagnostics. The map is guarded for
// concurrent reads; per-diagnostic sample writes are serialized by system
// scheduling (L1 §4.1), so the hot Push path takes only a read lock.
type DiagnosticsStore struct {
	mu      sync.RWMutex
	metrics map[DiagnosticPath]*Diagnostic
}

// NewDiagnosticsStore returns an empty store.
func NewDiagnosticsStore() *DiagnosticsStore {
	return &DiagnosticsStore{metrics: make(map[DiagnosticPath]*Diagnostic)}
}

// Register adds a diagnostic. A duplicate path is a Warning, not a crash
// (L1 §4.2): the existing diagnostic is kept.
func (s *DiagnosticsStore) Register(d *Diagnostic) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.metrics[d.Path]; exists {
		slog.Warn("diag: duplicate diagnostic path ignored", "path", string(d.Path))
		return
	}
	s.metrics[d.Path] = d
}

// Get returns the diagnostic registered at path.
func (s *DiagnosticsStore) Get(path DiagnosticPath) (*Diagnostic, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d, ok := s.metrics[path]
	return d, ok
}

// AddReader registers interest in a diagnostic, enabling its collection (INV-1).
// Called at setup, before the Push hot path runs.
func (s *DiagnosticsStore) AddReader(path DiagnosticPath) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if d, ok := s.metrics[path]; ok {
		d.readers++
	}
}

// Push records value for path. It is a no-op when the diagnostic is
// unregistered, disabled, or has zero readers — the per-metric half of the
// zero-cost-when-unread invariant (INV-1).
func (s *DiagnosticsStore) Push(path DiagnosticPath, value float64) {
	s.PushAt(path, value, 0)
}

// PushAt is Push with an explicit timestamp (informational; averages remain
// sample-count based).
func (s *DiagnosticsStore) PushAt(path DiagnosticPath, value float64, ts time.Duration) {
	s.mu.RLock()
	d, ok := s.metrics[path]
	s.mu.RUnlock()
	if !ok || !d.enabled || d.readers == 0 {
		return
	}
	d.record(value, ts)
}

// SetEnabled toggles collection for a diagnostic without unregistering it.
func (s *DiagnosticsStore) SetEnabled(path DiagnosticPath, enabled bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if d, ok := s.metrics[path]; ok {
		d.enabled = enabled
	}
}

// HasAnyReader reports whether any registered diagnostic has a reader. It is the
// run-condition that gates the entire collection pass to zero cost when nothing
// consumes diagnostics (INV-1).
func (s *DiagnosticsStore) HasAnyReader() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.metrics {
		if d.readers > 0 {
			return true
		}
	}
	return false
}

// Len returns the number of registered diagnostics.
func (s *DiagnosticsStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.metrics)
}
