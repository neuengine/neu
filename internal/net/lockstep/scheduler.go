package lockstep

import (
	"time"

	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/scene"
)


const (
	// DefaultInputDelay is the number of ticks of artificial delay added to local
	// input before it is executed. Hides small network jitter without stalling.
	DefaultInputDelay uint8 = 2
	// DefaultChecksumInterval is how often (in ticks) peers exchange CRC32
	// checksums for desync detection (INV-4).
	DefaultChecksumInterval uint64 = 30
	// DefaultInputTimeout is the duration after which a peer whose input has not
	// arrived is disconnected (INV-5 — never an infinite wait).
	DefaultInputTimeout = 5 * time.Second
	// stallThreshold is the wait before emitting the "waiting for players" signal.
	stallThreshold = 200 * time.Millisecond
)

// WaitingForPlayers is emitted (via OnStall) when the scheduler has been
// waiting for peers' inputs longer than stallThreshold.
type WaitingForPlayers struct {
	Tick  uint64
	Peers []netcore.PlayerID
}

// DesyncDetected is emitted (via OnDesync) when a peer's checksum for a tick
// differs from the local checksum. The scheduler halts on detection (INV-4).
type DesyncDetected struct {
	Tick uint64
	Peer netcore.PlayerID
}

// LockstepConfig holds the static parameters for LockstepScheduler.
type LockstepConfig struct {
	// Peers is the full set of player IDs whose input must arrive before each
	// tick advances. Includes the local player (INV-1).
	Peers []netcore.PlayerID
	// InputDelay is the number of ticks added to local input before execution.
	// Default: DefaultInputDelay.
	InputDelay uint8
	// ChecksumInterval controls how often CRC32 checksums are exchanged (INV-4).
	// Default: DefaultChecksumInterval.
	ChecksumInterval uint64
	// InputTimeout is the maximum wait for a peer's input before disconnect (INV-5).
	// Default: DefaultInputTimeout.
	InputTimeout time.Duration
}

// LockstepScheduler is the all-peers-ready tick gate. Its Step method advances
// the simulation by one tick only when every peer has submitted input for
// currentTick+InputDelay (INV-1). It drives DeterministicSchedule.RunTick (INV-2),
// never replicates state (INV-3), and exchanges CRC32 checksums every
// ChecksumInterval ticks for desync detection (INV-4). A peer that exceeds
// InputTimeout is disconnected, never waited on indefinitely (INV-5).
type LockstepScheduler struct {
	cfg         LockstepConfig
	currentTick uint64
	stallSince  time.Time
	halted      bool

	// OnStall is called when peers' input has been absent for stallThreshold.
	// May be nil.
	OnStall func(WaitingForPlayers)
	// OnDesync is called when a checksum mismatch is detected. The scheduler
	// halts after calling this (INV-4). May be nil.
	OnDesync func(DesyncDetected)
}

// NewLockstepScheduler returns a scheduler with the given config, applying
// defaults for zero fields.
func NewLockstepScheduler(cfg LockstepConfig) *LockstepScheduler {
	if cfg.InputDelay == 0 {
		cfg.InputDelay = DefaultInputDelay
	}
	if cfg.ChecksumInterval == 0 {
		cfg.ChecksumInterval = DefaultChecksumInterval
	}
	if cfg.InputTimeout == 0 {
		cfg.InputTimeout = DefaultInputTimeout
	}
	return &LockstepScheduler{cfg: cfg}
}

// CurrentTick returns the last fully simulated tick.
func (ls *LockstepScheduler) CurrentTick() uint64 { return ls.currentTick }

// Halted reports whether the scheduler has stopped due to a desync (INV-4).
func (ls *LockstepScheduler) Halted() bool { return ls.halted }

// Run wraps Step for ECS schedule registration, discarding the bool return.
func (ls *LockstepScheduler) Run(w *world.World) { _ = ls.Step(w) }

// Step attempts to advance one simulation tick. It returns true if the tick was
// executed, false if the scheduler is waiting for peer input or has been halted.
func (ls *LockstepScheduler) Step(w *world.World) bool {
	if ls.halted {
		return false
	}

	rcP, ok := world.Resource[*netcore.RollbackCoordinator](w)
	if !ok {
		return false
	}
	rc := *rcP

	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		return false
	}
	buf := *bufP

	snapsP, ok := world.Resource[*netcore.SnapshotManager](w)
	if !ok {
		return false
	}
	snaps := *snapsP

	target := ls.currentTick + uint64(ls.cfg.InputDelay)

	// Check that every peer has submitted input for target (INV-1).
	if !buf.HasAllPeers(target, ls.cfg.Peers) {
		ls.checkStallAndTimeout(w, target)
		return false
	}

	// All peers ready — reset stall clock and advance.
	ls.stallSince = time.Time{}
	ls.currentTick++

	reader := scene.NewWorldAdapter(w)
	rc.AdvanceTick(w, reader, ls.currentTick)

	// Periodic desync check (INV-4): broadcast local CRC32 to all peers.
	if ls.currentTick%ls.cfg.ChecksumInterval == 0 {
		ls.broadcastChecksum(w, snaps)
	}

	return true
}

// checkStallAndTimeout handles stall detection and peer timeout (INV-5).
func (ls *LockstepScheduler) checkStallAndTimeout(w *world.World, target uint64) {
	now := time.Now()
	if ls.stallSince.IsZero() {
		ls.stallSince = now
		return
	}
	elapsed := now.Sub(ls.stallSince)

	// Signal "waiting for players" once the stall threshold is crossed.
	if elapsed >= stallThreshold && ls.OnStall != nil {
		ls.OnStall(WaitingForPlayers{Tick: target, Peers: ls.cfg.Peers})
	}

	// Disconnect peers that have exceeded the input timeout (INV-5).
	if elapsed >= ls.cfg.InputTimeout {
		ls.disconnectLatePeers(w, target)
	}
}

// disconnectLatePeers disconnects any peer that has not submitted input for
// target and removes it from the peer list so the gate can proceed.
func (ls *LockstepScheduler) disconnectLatePeers(w *world.World, target uint64) {
	trP, ok := world.Resource[netcore.TransportResource](w)
	if !ok {
		return
	}
	tr := trP.T

	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		return
	}
	buf := *bufP

	remaining := ls.cfg.Peers[:0]
	for _, p := range ls.cfg.Peers {
		if _, has := buf.GetInput(target, p); has {
			remaining = append(remaining, p)
			continue
		}
		// Disconnect: map PlayerID to ConnectionID by identity convention.
		tr.Disconnect(netcore.ConnectionID(p), "input timeout")
	}
	ls.cfg.Peers = remaining
	ls.stallSince = time.Time{}
}

// ReceiveChecksum is called when a peer sends its CRC32 checksum for tick.
// If it differs from the local snapshot's checksum, the scheduler halts (INV-4).
func (ls *LockstepScheduler) ReceiveChecksum(w *world.World, peer netcore.PlayerID, tick uint64, peerChecksum uint32) {
	if ls.halted {
		return
	}
	snapsP, ok := world.Resource[*netcore.SnapshotManager](w)
	if !ok {
		return
	}
	snaps := *snapsP

	h, ok := snaps.Get(tick)
	if !ok {
		return // snapshot evicted; cannot verify
	}
	if h.Checksum != peerChecksum {
		ls.halted = true
		if ls.OnDesync != nil {
			ls.OnDesync(DesyncDetected{Tick: tick, Peer: peer})
		}
	}
}

// broadcastChecksum enqueues the local CRC32 for currentTick to all peers (INV-4).
// Uses the snapshot already taken by RollbackCoordinator.AdvanceTick.
func (ls *LockstepScheduler) broadcastChecksum(w *world.World, snaps *netcore.SnapshotManager) {
	h, ok := snaps.Get(ls.currentTick)
	if !ok {
		return
	}
	payload := encodeChecksumPacket(ls.currentTick, h.Checksum)
	if oq, ok2 := world.Resource[netcore.OutboundQueue](w); ok2 {
		oq.Messages = append(oq.Messages, netcore.OutboundMessage{
			Channel: netcore.ChannelEvents,
			Payload: payload,
		})
	}
}
