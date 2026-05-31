//go:build editor

package aiapi

import (
	"sync"
	"time"
)

// rateLimiter is a per-provider token bucket enforcing requests-per-minute and
// tokens-per-minute client-side (INV-8), in addition to the assistant manager's
// global limit. It is safe for concurrent use (INV-3).
type rateLimiter struct {
	mu sync.Mutex

	rpm, tpm   int
	reqTokens  float64 // available request permits
	tokTokens  float64 // available token permits
	lastRefill time.Time
	now        func() time.Time // injectable clock for tests
}

// newRateLimiter builds a limiter (rpm/tpm <= 0 disables that dimension).
func newRateLimiter(rpm, tpm int) *rateLimiter {
	return &rateLimiter{
		rpm: rpm, tpm: tpm,
		reqTokens:  float64(rpm),
		tokTokens:  float64(tpm),
		lastRefill: time.Now(),
		now:        time.Now,
	}
}

// refill adds permits proportional to elapsed time, capped at the per-minute max.
func (r *rateLimiter) refill() {
	now := r.now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	if elapsed <= 0 {
		return
	}
	r.lastRefill = now
	if r.rpm > 0 {
		r.reqTokens = min(float64(r.rpm), r.reqTokens+elapsed*float64(r.rpm)/60)
	}
	if r.tpm > 0 {
		r.tokTokens = min(float64(r.tpm), r.tokTokens+elapsed*float64(r.tpm)/60)
	}
}

// Allow reserves one request + estimatedTokens. It returns ok=false with a
// retry-after (seconds) when either bucket is exhausted (INV-8).
func (r *rateLimiter) Allow(estimatedTokens int) (ok bool, retryAfter int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.refill()

	if r.rpm > 0 && r.reqTokens < 1 {
		return false, retrySeconds(1-r.reqTokens, r.rpm)
	}
	if r.tpm > 0 && r.tokTokens < float64(estimatedTokens) {
		return false, retrySeconds(float64(estimatedTokens)-r.tokTokens, r.tpm)
	}
	if r.rpm > 0 {
		r.reqTokens--
	}
	if r.tpm > 0 {
		r.tokTokens -= float64(estimatedTokens)
	}
	return true, 0
}

// retrySeconds estimates how long until `deficit` permits refill at perMinute rate.
func retrySeconds(deficit float64, perMinute int) int {
	if perMinute <= 0 {
		return 0
	}
	s := int(deficit * 60 / float64(perMinute))
	if s < 1 {
		return 1
	}
	return s
}
