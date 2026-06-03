//go:build editor

package aiapi

import "github.com/neuengine/neu/pkg/diag"

// Diagnostic paths the plugin publishes per successful request (L1 §4.10, INV-9).
const (
	DiagTokensInput  diag.DiagnosticPath = "aiapi/tokens_input"
	DiagTokensOutput diag.DiagnosticPath = "aiapi/tokens_output"
	DiagCostUSD      diag.DiagnosticPath = "aiapi/cost_usd"
)

// UsageRecorder receives token/cost usage after every successful provider call
// (INV-9). It is an interface so the plugin stays decoupled from the concrete
// diagnostics store and tests can inject a counter. The engine wires a
// diag-backed recorder via NewDiagRecorder.
type UsageRecorder interface {
	RecordUsage(provider string, u Usage)
}

// RegisterDiagnostics registers the three aiapi usage metrics on the store so a
// reader can be attached (collection stays zero-cost until then, INV-1).
func RegisterDiagnostics(store *diag.DiagnosticsStore) {
	store.Register(diag.NewDiagnostic(DiagTokensInput, "tokens", 0))
	store.Register(diag.NewDiagnostic(DiagTokensOutput, "tokens", 0))
	store.Register(diag.NewDiagnostic(DiagCostUSD, "usd", 0))
}

// diagRecorder is the engine-side UsageRecorder backed by a DiagnosticsStore.
// Push is a no-op for any metric with no reader, so recording costs nothing
// until a cost dashboard / budget alert subscribes (INV-1).
type diagRecorder struct {
	store *diag.DiagnosticsStore
}

// NewDiagRecorder returns a UsageRecorder that pushes usage into store. It does
// not register the metrics; call RegisterDiagnostics once at plugin setup.
func NewDiagRecorder(store *diag.DiagnosticsStore) UsageRecorder {
	return &diagRecorder{store: store}
}

// RecordUsage pushes input/output token counts and cost for one request. The
// provider label is accepted for parity with the L1 metric tags; the current
// diag store is path-keyed (untagged), so the provider is recorded via the
// caller's logging rather than a tag dimension.
func (r *diagRecorder) RecordUsage(_ string, u Usage) {
	r.store.Push(DiagTokensInput, float64(u.InputTokens))
	r.store.Push(DiagTokensOutput, float64(u.OutputTokens))
	if u.CostUSD != 0 {
		r.store.Push(DiagCostUSD, u.CostUSD)
	}
}

var _ UsageRecorder = (*diagRecorder)(nil)
