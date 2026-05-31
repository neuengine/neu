//go:build editor

package aiapi

import (
	"fmt"
	"os"
	"strings"

	"github.com/neuengine/neu/pkg/errs"
)

// resolveSecret reads an API key from the source named in ProviderConfig
// (L1 §4.7). Supported sources: "env:NAME" (stdlib, default). "keyring:label"
// and "file:path" (age-encrypted) are ADR-gated and return ErrSecretSourceUnsupported
// until their deps land. The secret is never written back to its source.
func resolveSecret(source string) ([]byte, error) {
	scheme, ref, ok := strings.Cut(source, ":")
	if !ok {
		return nil, APIError{Code: CodeMissingKey, Message: "api_key_source must be scheme:ref"}
	}
	switch scheme {
	case "env":
		v := os.Getenv(ref)
		if v == "" {
			return nil, APIError{Code: CodeMissingKey, Message: fmt.Sprintf("env %q is empty", ref)}
		}
		return []byte(v), nil
	case "keyring", "file":
		return nil, APIError{Code: CodeMissingKey, Message: scheme + " source requires an opt-in dependency (ADR pending)"}
	default:
		return nil, APIError{Code: CodeMissingKey, Message: "unknown api_key_source scheme " + scheme}
	}
}

// newRedactor builds a credential redactor (INV-1) seeded with the resolved
// secret. It wraps the engine's errs.Redactor — log/error output routed through
// it masks the key as "***". The caller zeroes the secret bytes after use.
func newRedactor(secret []byte) *errs.Redactor {
	r := errs.NewRedactor("***")
	if len(secret) > 0 {
		r.AddSecret(string(secret))
	}
	return r
}

// zero overwrites a secret buffer in place (defence-in-depth after the request).
func zero(b []byte) {
	for i := range b {
		b[i] = 0
	}
}
