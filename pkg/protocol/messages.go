// Package protocol defines the versioned IPC wire messages used for
// engine↔editor communication. All message types serialize as newline-delimited
// JSON (one JSON object per line). Import list is restricted to stdlib only
// (INV-4 of l2-multi-repo-architecture-go.md).
package protocol

// Kind is the discriminator string carried in the "type" JSON field of every
// message. Unknown kinds are skipped gracefully by Decode (forward-compat rule).
type Kind string

const (
	KindHotReloadPrepare Kind = "HotReloadPrepare"
	KindHotReloadReady   Kind = "HotReloadReady"
	KindHotReloadFailed  Kind = "HotReloadFailed"
	KindShaderError      Kind = "ShaderError"
	KindShaderReloaded   Kind = "ShaderReloaded"
	KindReloadMetrics    Kind = "ReloadMetrics"
	KindNetworkAlert     Kind = "NetworkAlert"
	KindDiagnosticSnap   Kind = "DiagnosticSnapshot"
)

// AlertLevel classifies the severity of a NetworkAlert.
type AlertLevel uint8

const (
	AlertWarning  AlertLevel = iota
	AlertCritical AlertLevel = iota
)

// HotReloadPrepare signals the editor to pause before state capture begins.
type HotReloadPrepare struct {
	Type         Kind   `json:"type"`
	SnapshotPath string `json:"snapshot_path"`
}

// HotReloadReady signals that state capture completed successfully.
type HotReloadReady struct {
	Type         Kind   `json:"type"`
	SnapshotPath string `json:"snapshot_path"`
	EntityCount  uint32 `json:"entity_count"`
	SnapshotSize uint64 `json:"snapshot_size"`
}

// HotReloadFailed signals that state capture or compilation failed.
type HotReloadFailed struct {
	Type   Kind   `json:"type"`
	Reason string `json:"reason"`
}

// ReloadMetrics carries timing breakdowns for a completed hot-reload cycle.
type ReloadMetrics struct {
	Type         Kind     `json:"type"`
	SnapshotMS   int64    `json:"snapshot_ms"`
	BuildMS      int64    `json:"build_ms"`
	RestoreMS    int64    `json:"restore_ms"`
	EntitiesLost []string `json:"entities_lost"`
}

// ShaderError reports a runtime shader compilation error.
type ShaderError struct {
	Type    Kind   `json:"type"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ShaderReloaded confirms successful shader hot-reload.
type ShaderReloaded struct {
	Type Kind   `json:"type"`
	Path string `json:"path"`
}

// NetworkAlert reports a metric threshold crossing.
type NetworkAlert struct {
	Type    Kind       `json:"type"`
	Metric  string     `json:"metric"`
	Level   AlertLevel `json:"level"`
	Value   float64    `json:"value"`
	Message string     `json:"message"`
}

// DiagnosticSnapshot is a point-in-time metrics dump from the DiagnosticsStore.
type DiagnosticSnapshot struct {
	Type      Kind               `json:"type"`
	Timestamp int64              `json:"timestamp"`
	Metrics   map[string]float64 `json:"metrics"`
}
