package errs

import (
	"io"
	"strings"
	"sync"
)

// Redactor replaces registered secret substrings with a mask before text
// reaches a log or transport. It is used by API-key-handling plugins (e.g. the
// AI API plugin) to keep credentials out of error messages and diagnostics.
type Redactor struct {
	mu      sync.RWMutex
	secrets []string
	mask    string
}

// NewRedactor returns a Redactor using the given mask ("" defaults to "***").
func NewRedactor(mask string) *Redactor {
	if mask == "" {
		mask = "***"
	}
	return &Redactor{mask: mask}
}

// AddSecret registers a literal secret to mask. Empty strings are ignored so a
// blank secret can never blanket-redact all output.
func (r *Redactor) AddSecret(secret string) {
	if secret == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.secrets = append(r.secrets, secret)
}

// Redact returns s with every registered secret replaced by the mask.
func (r *Redactor) Redact(s string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, secret := range r.secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, r.mask)
		}
	}
	return s
}

// Writer wraps w so every Write is redacted first. Useful as a structured-log
// or stderr sink that must never leak credentials.
func (r *Redactor) Writer(w io.Writer) io.Writer {
	return &redactWriter{r: r, w: w}
}

type redactWriter struct {
	r *Redactor
	w io.Writer
}

// Write redacts p and forwards it. It reports len(p) consumed on success so it
// satisfies io.Writer's contract even though the masked bytes may differ in
// length from the input.
func (rw *redactWriter) Write(p []byte) (int, error) {
	if _, err := io.WriteString(rw.w, rw.r.Redact(string(p))); err != nil {
		return 0, err
	}
	return len(p), nil
}
