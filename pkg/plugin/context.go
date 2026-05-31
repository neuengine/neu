package plugin

import "log/slog"

// CommandIssuer is the capability-checked command surface a plugin uses to
// mutate the world. Every command it issues is tagged with the plugin ID and
// flows through the engine's standard CommandBuffer (INV-7) — there is no
// *World accessor. Concrete in-process and out-of-process issuers share this
// contract; the minimal surface here grows with the loader implementation.
type CommandIssuer interface {
	// PluginID returns the owning plugin, stamped on every issued command.
	PluginID() PluginID
}

// PluginContext is the capability-checked handle the engine passes to a plugin
// during its lifecycle. It exposes only deferred mutation + read access — no
// direct world handle (INV-7). The engine supplies a capability-enforcing
// implementation (INV-3).
type PluginContext interface {
	// Commands returns the plugin-ID-tagged command issuer.
	Commands() CommandIssuer
	// Config returns the user config validated against the manifest schema.
	Config() []byte
	// Capabilities returns the set granted to this plugin.
	Capabilities() CapabilitySet
	// Logger returns a slog logger scoped to the plugin.
	Logger() *slog.Logger
	// Check returns a CapabilityError (wrapping ErrCapabilityDenied) if the
	// capability is not granted — the runtime enforcement point for INV-3.
	Check(c Capability) error
}
