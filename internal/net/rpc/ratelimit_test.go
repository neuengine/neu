package rpc

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/app"
)

// ─── TokenBucket ──────────────────────────────────────────────────────────────

func TestTokenBucketAllowsUpToCapacity(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := NewTokenBucket(3, 10, now)

	for i := range 3 {
		if !b.Allow(now) {
			t.Errorf("token %d should be allowed", i)
		}
	}
	if b.Allow(now) {
		t.Error("4th token should be denied (capacity=3)")
	}
}

func TestTokenBucketRefillsOverTime(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := NewTokenBucket(1, 10, now) // 10 tokens/s, cap=1

	b.Allow(now) // drain the 1 token

	// 0.15s later → 1.5 tokens refilled, clamped to cap=1.
	later := now.Add(150 * time.Millisecond)
	if !b.Allow(later) {
		t.Error("bucket should have refilled after 150ms")
	}
}

func TestTokenBucketNoRefillWithoutTime(t *testing.T) {
	t.Parallel()
	now := time.Now()
	b := NewTokenBucket(1, 1000, now)
	b.Allow(now) // drain
	// Same time → no refill.
	if b.Allow(now) {
		t.Error("no refill should happen at same timestamp")
	}
}

// ─── RpcRateLimit ─────────────────────────────────────────────────────────────

func TestRpcRateLimitAllowsUpToGlobalRate(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rl := NewRpcRateLimit(3) // 3 tokens/s, capacity=3
	const conn netcore.ConnectionID = 1

	// First 3 RPCs allowed.
	for i := range 3 {
		if !rl.Allow(conn, 0, now) {
			t.Errorf("Allow %d should succeed", i)
		}
	}
	// 4th denied.
	if rl.Allow(conn, 0, now) {
		t.Error("4th Allow should be denied")
	}
}

func TestRpcRateLimitPerTypeOverride(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rl := NewRpcRateLimit(100) // high global
	rl.SetTypeLimit(7, 1)     // typeID 7: only 1/s
	const conn netcore.ConnectionID = 2

	if !rl.Allow(conn, 7, now) {
		t.Error("first Allow for typeID 7 should succeed")
	}
	if rl.Allow(conn, 7, now) {
		t.Error("second Allow for typeID 7 should be denied")
	}
	// Global limit for other types still high.
	for i := range 5 {
		if !rl.Allow(conn, 1, now) {
			t.Errorf("global limit: Allow %d for typeID 1 should succeed", i)
		}
	}
}

func TestRpcRateLimitIsolatedPerConnection(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rl := NewRpcRateLimit(1)
	const c1 netcore.ConnectionID = 10
	const c2 netcore.ConnectionID = 11

	rl.Allow(c1, 0, now) // drain c1
	// c2 must be independent — still has its own full bucket.
	if !rl.Allow(c2, 0, now) {
		t.Error("c2 should have its own independent bucket")
	}
	if rl.Allow(c1, 0, now) {
		t.Error("c1 should still be drained")
	}
}

func TestRpcRateLimitRecordDropAndCount(t *testing.T) {
	t.Parallel()
	rl := NewRpcRateLimit(0) // default 100/s
	const conn netcore.ConnectionID = 5
	rl.RecordDrop(conn)
	rl.RecordDrop(conn)
	if got := rl.Drops(conn); got != 2 {
		t.Errorf("Drops = %d, want 2", got)
	}
}

func TestRpcRateLimitForgetConnection(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rl := NewRpcRateLimit(1)
	const conn netcore.ConnectionID = 3
	rl.Allow(conn, 0, now) // creates bucket
	rl.RecordDrop(conn)
	rl.ForgetConnection(conn)

	// After forget, drops reset.
	if rl.Drops(conn) != 0 {
		t.Error("Drops should be 0 after ForgetConnection")
	}
	// New bucket created on next Allow.
	if !rl.Allow(conn, 0, now) {
		t.Error("Allow after ForgetConnection should succeed (fresh bucket)")
	}
}

func TestRpcRateLimitDefaultRate(t *testing.T) {
	t.Parallel()
	rl := NewRpcRateLimit(0) // 0 → default 100/s
	now := time.Now()
	const conn netcore.ConnectionID = 99
	// Should allow 100 messages immediately (capacity = 100).
	for i := range 100 {
		if !rl.Allow(conn, 0, now) {
			t.Errorf("allow %d should succeed at default rate (100/s, cap=100)", i)
		}
	}
	// 101st denied.
	if rl.Allow(conn, 0, now) {
		t.Error("101st should be denied")
	}
}

// ─── RpcPlugin ────────────────────────────────────────────────────────────────

func TestRpcPluginRegistersResources(t *testing.T) {
	t.Parallel()
	a := app.NewApp()
	a.AddPlugin(RpcPlugin{})
	w := a.World()

	if _, ok := world.Resource[*RpcRegistry](w); !ok {
		t.Error("*RpcRegistry not registered by RpcPlugin")
	}
	if _, ok := world.Resource[*RpcRateLimit](w); !ok {
		t.Error("*RpcRateLimit not registered by RpcPlugin")
	}
}

func TestRpcPluginUsesProvidedRegistry(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	a := app.NewApp()
	a.AddPlugin(RpcPlugin{Registry: reg})
	w := a.World()

	rrp, ok := world.Resource[*RpcRegistry](w)
	if !ok || rrp == nil || *rrp != reg {
		t.Error("RpcPlugin should use the provided registry, not create a new one")
	}
}

func TestRpcPluginCustomGlobalRateLimit(t *testing.T) {
	t.Parallel()
	a := app.NewApp()
	a.AddPlugin(RpcPlugin{GlobalRateLimit: 5})
	w := a.World()

	rlp, ok := world.Resource[*RpcRateLimit](w)
	if !ok || rlp == nil {
		t.Fatal("*RpcRateLimit not registered")
	}
	rl := *rlp
	// Rate=5 means capacity=5; first 5 calls should succeed.
	now := time.Now()
	const conn netcore.ConnectionID = 1
	for i := range 5 {
		if !rl.Allow(conn, 0, now) {
			t.Errorf("allow %d should succeed at rate 5", i)
		}
	}
	if rl.Allow(conn, 0, now) {
		t.Error("6th allow should be denied at rate 5")
	}
}

// TestRpcPluginReceiveSystemDropsOverLimit verifies integration between the
// plugin and rate limiter: over-limit packets are dropped before dispatch.
func TestRpcPluginReceiveSystemDropsOverLimit(t *testing.T) {
	t.Parallel()
	reg := NewRpcRegistry()
	w := world.NewWorld()
	def, _ := RegisterRpc[MoveCmd](reg, w, DirClientToServer, 0)

	rl := NewRpcRateLimit(0) // default 100/s, capacity=100
	// Drain the bucket for connection 1 completely.
	now := time.Now()
	for range 100 {
		rl.Allow(1, def.TypeID, now)
	}
	world.SetResource(w, rl)

	payload, _ := json.Marshal(MoveCmd{X: 1})
	pkt := EncodeRpcMessage(def.TypeID, payload)
	world.SetResource(w, netcore.InboundQueue{Packets: []netcore.InboundPacket{
		{Connection: 1, Channel: 0, Payload: pkt},
	}})

	sys := NewRpcReceiveSystem(reg)
	sys.Run(w)

	if rl.Drops(1) != 1 {
		t.Errorf("Drops = %d, want 1 (rate-limited message)", rl.Drops(1))
	}
}
