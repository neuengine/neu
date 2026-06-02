package protocol

// OOP plugin lifecycle wire messages. The engine and the out-of-process plugin
// binary exchange these over the stdio transport (newline-delimited JSON).
//
// Handshake sequence:
//
//	engine → plugin : PluginInit   (engine version, granted capabilities)
//	plugin → engine : PluginHello  (plugin id, declared methods, protocol version)
//
// Lifecycle sequence (repeats for Build, Ready, Finish, Cleanup):
//
//	engine → plugin : PluginLifecycle{Phase: "Build"}
//	plugin → engine : PluginLifecycleDone{Phase: "Build"}   (or PluginError)

const (
	KindPluginInit          Kind = "PluginInit"
	KindPluginHello         Kind = "PluginHello"
	KindPluginLifecycle     Kind = "PluginLifecycle"
	KindPluginLifecycleDone Kind = "PluginLifecycleDone"
	KindPluginError         Kind = "PluginError"
)

// LifecyclePhase names the plugin phase being driven.
type LifecyclePhase string

const (
	PhaseBuild   LifecyclePhase = "Build"
	PhaseReady   LifecyclePhase = "Ready"
	PhaseFinish  LifecyclePhase = "Finish"
	PhaseCleanup LifecyclePhase = "Cleanup"
)

// PluginInit is sent by the engine immediately after spawning the subprocess.
// It carries the running engine version and the granted capability set so the
// plugin can self-validate before executing any code.
type PluginInit struct {
	Type          Kind     `json:"type"`
	EngineVersion string   `json:"engine_version"`
	GrantedCaps   []string `json:"granted_caps"`
}

// PluginHello is the subprocess's reply to PluginInit. It declares the plugin's
// identity and the protocol version it speaks.
type PluginHello struct {
	Type            Kind   `json:"type"`
	ID              string `json:"id"`
	Version         string `json:"version"`
	ProtocolVersion string `json:"protocol_version"`
}

// PluginLifecycle is sent by the engine to advance the plugin to the next
// lifecycle phase (Build, Ready, Finish, or Cleanup). The plugin executes the
// corresponding logic and replies with PluginLifecycleDone.
type PluginLifecycle struct {
	Type  Kind           `json:"type"`
	Phase LifecyclePhase `json:"phase"`
}

// PluginLifecycleDone is the plugin's acknowledgement that the requested phase
// completed successfully.
type PluginLifecycleDone struct {
	Type  Kind           `json:"type"`
	Phase LifecyclePhase `json:"phase"`
}

// PluginError carries a structured failure from the plugin subprocess. Sent
// instead of PluginLifecycleDone when a phase fails, or proactively when the
// plugin detects a fatal error.
type PluginError struct {
	Type    Kind           `json:"type"`
	Phase   LifecyclePhase `json:"phase,omitempty"`
	Message string         `json:"message"`
	Code    int            `json:"code,omitempty"`
}
