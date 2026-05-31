package plugin

import (
	"errors"
	"fmt"
)

// Sentinel errors for the plugin distribution pipeline. They map to the
// E-PLUGIN-{NNN} taxonomy (l1-error-core); callers match with errors.Is.
var (
	// ErrManifestInvalid: missing or schema-nonconformant manifest (E-PLUGIN-001).
	ErrManifestInvalid = errors.New("plugin: manifest invalid")
	// ErrEngineIncompatible: engine_version constraint not satisfied (E-PLUGIN-002).
	ErrEngineIncompatible = errors.New("plugin: engine version constraint not satisfied")
	// ErrCapabilityDenied: a runtime operation outside the granted set (E-PLUGIN-003).
	ErrCapabilityDenied = errors.New("plugin: capability not granted")
	// ErrDuplicateID: a second plugin with an already-registered ID (E-PLUGIN-004).
	ErrDuplicateID = errors.New("plugin: duplicate plugin id")
	// ErrChecksumMismatch: installed manifest altered after install (E-PLUGIN-005).
	ErrChecksumMismatch = errors.New("plugin: manifest checksum mismatch")
)

// CapabilityError wraps ErrCapabilityDenied with the offending plugin +
// capability for audit logging (L1 INV-3: rejections logged with site).
type CapabilityError struct {
	Plugin     PluginID
	Capability Capability
}

func (e CapabilityError) Error() string {
	return fmt.Sprintf("plugin %q: capability %q not granted", string(e.Plugin), string(e.Capability))
}

func (e CapabilityError) Unwrap() error { return ErrCapabilityDenied }
