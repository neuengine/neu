//go:build editor

package aiapi

import "time"

// ProviderConfig configures one provider endpoint (L1 §4.3).
type ProviderConfig struct {
	Endpoint      string            `json:"endpoint"`
	Model         string            `json:"model"`
	APIKeySource  string            `json:"api_key_source"` // "env:NAME" | "keyring:label" | "file:path"
	OrganizationID string           `json:"organization_id,omitempty"`
	ExtraHeaders  map[string]string `json:"extra_headers,omitempty"`
}

// Config is the plugin's user configuration (L1 §4.3), validated against the
// manifest schema and passed via PluginContext.Config().
type Config struct {
	ActiveProvider string                    `json:"active_provider"`
	Providers      map[string]ProviderConfig `json:"providers"`
	DefaultTimeout time.Duration             `json:"default_timeout"`
	RateLimitRPM   int                       `json:"rate_limit_rpm"`
	RateLimitTPM   int                       `json:"rate_limit_tpm"`
	RedactLogs     bool                      `json:"redact_logs"`
	CostBudgetUSD  float64                   `json:"cost_budget_usd,omitempty"`
}

// DefaultConfig returns sensible defaults (L1 §4.3): 30 s timeout, 60 RPM,
// 90000 TPM, redaction on.
func DefaultConfig() Config {
	return Config{
		Providers:      map[string]ProviderConfig{},
		DefaultTimeout: 30 * time.Second,
		RateLimitRPM:   60,
		RateLimitTPM:   90000,
		RedactLogs:     true,
	}
}
