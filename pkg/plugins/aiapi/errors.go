//go:build editor

package aiapi

import (
	"fmt"
	"net/http"
)

// Code is an E-PLUGIN-AIAPI-{NNN} error code (L1 §4.8). A finite, documented set
// maps every provider/HTTP failure (INV-5); unknown shapes fall through to 999.
type Code int

const (
	CodeMissingKey     Code = 100 // missing or invalid API key
	CodeUnsupported    Code = 101 // unsupported provider
	CodeHTTP4xx        Code = 200 // HTTP 4xx (excluding 401, 429)
	CodeUnauthorized   Code = 201 // HTTP 401
	CodeRateLimited    Code = 202 // HTTP 429 (+ retry-after)
	CodeHTTP5xx        Code = 300 // HTTP 5xx
	CodeTimeout        Code = 400 // request timeout
	CodeCancelled      Code = 401 // request cancelled
	CodeParse          Code = 500 // canonical translation parse error
	CodeCapabilityDeny Code = 600 // capability denied at runtime
	CodeUnknown        Code = 999 // unknown provider error shape
)

// APIError is a coded plugin error carrying the E-PLUGIN-AIAPI code.
type APIError struct {
	Code       Code
	Message    string
	RetryAfter int // seconds; set for CodeRateLimited
}

func (e APIError) Error() string {
	return fmt.Sprintf("E-PLUGIN-AIAPI-%03d: %s", int(e.Code), e.Message)
}

// ErrUnsupportedProvider is returned when a configured provider is not registered.
type ErrUnsupportedProvider struct{ Name string }

func (e ErrUnsupportedProvider) Error() string {
	return fmt.Sprintf("E-PLUGIN-AIAPI-%03d: unsupported provider %q", int(CodeUnsupported), e.Name)
}

// MapHTTPStatus maps an HTTP status code to the corresponding APIError code
// (INV-5). 2xx is not an error and returns ok=false.
func MapHTTPStatus(status int) (Code, bool) {
	switch {
	case status >= 200 && status < 300:
		return 0, false
	case status == http.StatusUnauthorized:
		return CodeUnauthorized, true
	case status == http.StatusTooManyRequests:
		return CodeRateLimited, true
	case status >= 400 && status < 500:
		return CodeHTTP4xx, true
	case status >= 500:
		return CodeHTTP5xx, true
	default:
		return CodeUnknown, true
	}
}
