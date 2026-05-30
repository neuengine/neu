// Package errs implements the engine's structured error taxonomy: a coded,
// categorized, localizable EngineError that composes with the standard errors
// package (Is/As/Unwrap). Error text lives in an external fs.FS-backed catalog
// so the same code drives a developer tooltip, a CI log line, and a localized
// player dialog.
//
// Bootstrap: l2-error-core-go Draft (Phase 6 Track K, C29 gate open).
package errs

// Severity classifies how an error affects engine execution (L1 §2.1).
type Severity uint8

const (
	// SeverityDebug is fine-grained development trace detail.
	SeverityDebug Severity = iota
	// SeverityWarning is a potential issue that does not stop execution.
	SeverityWarning
	// SeverityRecoverable is an error the caller can handle (e.g. entity not found).
	SeverityRecoverable
	// SeverityFatal is unrecoverable engine state requiring termination.
	SeverityFatal
)

// String renders the severity. The switch is total: every defined value has a
// case so a newly added level cannot be silently mis-rendered as a fallthrough.
func (s Severity) String() string {
	switch s {
	case SeverityDebug:
		return "Debug"
	case SeverityWarning:
		return "Warning"
	case SeverityRecoverable:
		return "Recoverable"
	case SeverityFatal:
		return "Fatal"
	default:
		return "Severity(?)"
	}
}

// Audience identifies who an error is addressed to (L1 §2.2). It lets
// diagnostics route developer errors to the editor and user errors to dialogs.
type Audience uint8

const (
	// AudienceDeveloper marks API misuse (e.g. querying an unregistered component).
	AudienceDeveloper Audience = iota
	// AudienceUser marks runtime issues from end-user input or malformed assets.
	AudienceUser
	// AudienceSystem marks OS/hardware issues (out of memory, file not found).
	AudienceSystem
)

// String renders the audience. Total switch (see Severity.String).
func (a Audience) String() string {
	switch a {
	case AudienceDeveloper:
		return "Developer"
	case AudienceUser:
		return "User"
	case AudienceSystem:
		return "System"
	default:
		return "Audience(?)"
	}
}
