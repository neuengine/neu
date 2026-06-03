//go:build editor

package aiapi

import (
	"testing"

	"github.com/neuengine/neu/pkg/diag"
)

func TestRegisterDiagnostics(t *testing.T) {
	t.Parallel()
	store := diag.NewDiagnosticsStore()
	RegisterDiagnostics(store)
	if store.Len() != 3 {
		t.Fatalf("registered %d diagnostics, want 3", store.Len())
	}
	for _, p := range []diag.DiagnosticPath{DiagTokensInput, DiagTokensOutput, DiagCostUSD} {
		if _, ok := store.Get(p); !ok {
			t.Errorf("diagnostic %q not registered", p)
		}
	}
}

func latest(t *testing.T, store *diag.DiagnosticsStore, p diag.DiagnosticPath) (float64, bool) {
	t.Helper()
	d, ok := store.Get(p)
	if !ok {
		return 0, false
	}
	e, ok := d.Latest()
	return e.Value, ok
}

func TestDiagRecorder_RecordsWithReader(t *testing.T) {
	t.Parallel()
	store := diag.NewDiagnosticsStore()
	RegisterDiagnostics(store)
	store.AddReader(DiagTokensInput)
	store.AddReader(DiagTokensOutput)
	store.AddReader(DiagCostUSD)

	rec := NewDiagRecorder(store)
	rec.RecordUsage("openai", Usage{InputTokens: 12, OutputTokens: 34, CostUSD: 0.5})

	if v, ok := latest(t, store, DiagTokensInput); !ok || v != 12 {
		t.Errorf("tokens_input = %v,%v want 12", v, ok)
	}
	if v, ok := latest(t, store, DiagTokensOutput); !ok || v != 34 {
		t.Errorf("tokens_output = %v,%v want 34", v, ok)
	}
	if v, ok := latest(t, store, DiagCostUSD); !ok || v != 0.5 {
		t.Errorf("cost_usd = %v,%v want 0.5", v, ok)
	}
}

func TestDiagRecorder_ZeroCostWhenNoReader(t *testing.T) {
	t.Parallel()
	// INV-1: with no reader registered, Push is a no-op — nothing recorded.
	store := diag.NewDiagnosticsStore()
	RegisterDiagnostics(store)
	rec := NewDiagRecorder(store)
	rec.RecordUsage("openai", Usage{InputTokens: 5, OutputTokens: 7, CostUSD: 1})

	if d, _ := store.Get(DiagTokensInput); d.Len() != 0 {
		t.Error("no reader ⇒ no collection (INV-1)")
	}
}

func TestDiagRecorder_SkipsZeroCost(t *testing.T) {
	t.Parallel()
	store := diag.NewDiagnosticsStore()
	RegisterDiagnostics(store)
	store.AddReader(DiagCostUSD)
	rec := NewDiagRecorder(store)
	rec.RecordUsage("openai", Usage{InputTokens: 1, OutputTokens: 1}) // CostUSD == 0

	if d, _ := store.Get(DiagCostUSD); d.Len() != 0 {
		t.Error("zero cost should not be pushed")
	}
}
