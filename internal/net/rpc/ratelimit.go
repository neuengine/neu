package rpc

import (
	"sync"
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

const defaultGlobalRateLimit = 100.0 // tokens per second per connection

// TokenBucket implements a token-bucket rate limiter.
// Tokens refill at refillRate per second up to capacity.
// Each Allow call consumes one token; returns false when empty.
type TokenBucket struct {
	tokens     float64
	capacity   float64
	refillRate float64
	lastRefill time.Time
}

// NewTokenBucket creates a bucket with the given capacity and refill rate (tokens/s).
func NewTokenBucket(capacity, refillRate float64, now time.Time) *TokenBucket {
	return &TokenBucket{
		tokens:     capacity,
		capacity:   capacity,
		refillRate: refillRate,
		lastRefill: now,
	}
}

// Allow attempts to consume one token. Returns true when a token was available.
// now must be monotonically non-decreasing across calls on the same bucket.
func (b *TokenBucket) Allow(now time.Time) bool {
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens = min(b.capacity, b.tokens+elapsed*b.refillRate)
		b.lastRefill = now
	}
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// connectionBuckets holds the rate-limiting state for one connection.
type connectionBuckets struct {
	global  *TokenBucket
	perType map[RpcTypeID]*TokenBucket
}

// RpcRateLimit enforces a token-bucket rate limit on inbound RPC messages,
// protecting the server from flooding clients (server-side only).
//
// Limits are applied per-connection with an optional per-type override.
// Over-limit messages are dropped and counted.
type RpcRateLimit struct {
	globalRate float64            // default tokens/s per connection
	perType    map[RpcTypeID]float64
	buckets    map[netcore.ConnectionID]*connectionBuckets
	drops      map[netcore.ConnectionID]uint64
	mu         sync.Mutex
}

// NewRpcRateLimit creates a rate limiter with the given global limit.
// A zero or negative globalRate uses the default (100 RPC/s per connection).
func NewRpcRateLimit(globalRate float64) *RpcRateLimit {
	if globalRate <= 0 {
		globalRate = defaultGlobalRateLimit
	}
	return &RpcRateLimit{
		globalRate: globalRate,
		perType:    make(map[RpcTypeID]float64),
		buckets:    make(map[netcore.ConnectionID]*connectionBuckets),
		drops:      make(map[netcore.ConnectionID]uint64),
	}
}

// SetTypeLimit registers a per-type rate limit (tokens/s) for typeID.
// Overrides the global limit for that type ID.
func (r *RpcRateLimit) SetTypeLimit(typeID RpcTypeID, rate float64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.perType[typeID] = rate
}

// Allow checks whether the connection connID may send an RPC of typeID right now.
// Returns true when within limit; false when the rate limit is exceeded.
// now should be time.Now() on the caller's side.
func (r *RpcRateLimit) Allow(connID netcore.ConnectionID, typeID RpcTypeID, now time.Time) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	cb := r.bucketsFor(connID, now)

	// Per-type limit takes priority over global.
	if rate, hasType := r.perType[typeID]; hasType {
		tb, ok := cb.perType[typeID]
		if !ok {
			tb = NewTokenBucket(rate, rate, now)
			cb.perType[typeID] = tb
		}
		return tb.Allow(now)
	}
	return cb.global.Allow(now)
}

// RecordDrop increments the drop counter for connID (called when Allow returns false).
func (r *RpcRateLimit) RecordDrop(connID netcore.ConnectionID) {
	r.mu.Lock()
	r.drops[connID]++
	r.mu.Unlock()
}

// Drops returns the total number of over-limit messages dropped for connID.
func (r *RpcRateLimit) Drops(connID netcore.ConnectionID) uint64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.drops[connID]
}

// ForgetConnection removes all state for connID (call on disconnect).
func (r *RpcRateLimit) ForgetConnection(connID netcore.ConnectionID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.buckets, connID)
	delete(r.drops, connID)
}

// bucketsFor returns or initializes connection buckets. Must be called with r.mu held.
func (r *RpcRateLimit) bucketsFor(connID netcore.ConnectionID, now time.Time) *connectionBuckets {
	cb, ok := r.buckets[connID]
	if !ok {
		cb = &connectionBuckets{
			global:  NewTokenBucket(r.globalRate, r.globalRate, now),
			perType: make(map[RpcTypeID]*TokenBucket),
		}
		r.buckets[connID] = cb
	}
	return cb
}
