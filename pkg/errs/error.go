package errs

import "errors"

// EngineError is the structured error contract (L1 §3). It embeds error so it
// flows unchanged through any error-typed return value, while exposing the
// code, severity, module, and solution for branching and localization.
type EngineError interface {
	error
	Code() Code
	Severity() Severity
	Module() string
	Solution() string
}

// engineError is the sole built-in EngineError implementation.
type engineError struct {
	code  Code
	args  []any
	cause error      // wrap chain target; nil for a root error
	trace []uintptr  // populated only in debug builds (INV trace)
	desc  Descriptor // resolved from the registry at construction; zero if unregistered
}

// New builds an EngineError for a registered code. args feed the localized
// message template. An unregistered code still produces a usable error whose
// message falls back to the code itself.
func New(code Code, args ...any) EngineError {
	d, _ := Lookup(code)
	return &engineError{code: code, args: args, desc: d, trace: captureTrace()}
}

// Wrap builds an EngineError that wraps cause; Unwrap returns cause so the
// standard errors.Is/As chain traverses into it.
func Wrap(code Code, cause error, args ...any) EngineError {
	d, _ := Lookup(code)
	return &engineError{code: code, args: args, cause: cause, desc: d, trace: captureTrace()}
}

// Error renders the localized message via the default catalog, appending the
// wrapped cause when present. It never returns an empty string (INV fallback).
func (e *engineError) Error() string {
	msg := defaultCatalog.Localize(e.code, e.args...)
	if e.cause != nil {
		return msg + ": " + e.cause.Error()
	}
	return msg
}

// Code returns the error's E-series code.
func (e *engineError) Code() Code { return e.code }

// Severity returns the registered severity (SeverityDebug if unregistered).
func (e *engineError) Severity() Severity { return e.desc.Severity }

// Module returns the registered owning module ("" if unregistered).
func (e *engineError) Module() string { return e.desc.Module }

// Solution returns the registered actionable advice ("" if unregistered).
func (e *engineError) Solution() string { return e.desc.Solution }

// Unwrap exposes the wrapped cause for errors.Is/As.
func (e *engineError) Unwrap() error { return e.cause }

// Is matches another error by code: errors.Is(err, New(code)) is true when the
// chain contains an engineError with the same code.
func (e *engineError) Is(target error) bool {
	t, ok := target.(*engineError)
	return ok && t.code == e.code
}

// Trace returns the captured program counters (empty in release builds).
func (e *engineError) Trace() []uintptr { return e.trace }

// MustSucceed panics if err is a Fatal developer EngineError — the only
// permitted panic path (L1 §5 Panic Policy). Any other error (including a
// non-EngineError) is returned to the caller; nil passes through.
func MustSucceed(err error) error {
	if err == nil {
		return nil
	}
	var ee EngineError
	if errors.As(err, &ee) && ee.Severity() == SeverityFatal && isDeveloper(ee) {
		panic(err)
	}
	return err
}

// isDeveloper reports whether ee targets the developer audience.
func isDeveloper(ee EngineError) bool {
	if c, ok := ee.(*engineError); ok {
		return c.desc.Audience == AudienceDeveloper
	}
	return false
}
