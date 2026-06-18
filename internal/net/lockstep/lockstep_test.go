package lockstep

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
)

// ─── mock transport ──────────────────────────────────────────────────────────

type mockTransport struct {
	disconnected []netcore.ConnectionID
	broadcasts   [][]byte
}

func (m *mockTransport) Listen(_ netcore.SocketAddr) error                 { return nil }
func (m *mockTransport) Connect(_ netcore.SocketAddr) (netcore.ConnectionID, error) {
	return 0, nil
}
func (m *mockTransport) Disconnect(id netcore.ConnectionID, _ string) {
	m.disconnected = append(m.disconnected, id)
}
func (m *mockTransport) Connections() []netcore.ConnectionID { return nil }
func (m *mockTransport) Send(_ netcore.ConnectionID, _ netcore.ChannelID, _ []byte) error {
	return nil
}
func (m *mockTransport) Broadcast(_ netcore.ChannelID, payload []byte) {
	m.broadcasts = append(m.broadcasts, payload)
}
func (m *mockTransport) Drain() []netcore.InboundPacket { return nil }
func (m *mockTransport) Stats(_ netcore.ConnectionID) netcore.ConnectionStats {
	return netcore.ConnectionStats{}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func buildLockstepWorld(t *testing.T) (*world.World, *mockTransport) {
	t.Helper()
	w := world.NewWorld()
	reg := typereg.NewTypeRegistry()
	snaps := netcore.NewSnapshotManager(reg, 16)
	sched := netcore.NewDeterministicSchedule(0, time.Millisecond)
	rc := netcore.NewRollbackCoordinator(snaps, sched)
	buf := netcore.NewInputBuffer()
	tr := &mockTransport{}
	world.SetResource(w, snaps)
	world.SetResource(w, sched)
	world.SetResource(w, rc)
	world.SetResource(w, buf)
	world.SetResource(w, netcore.InboundQueue{})
	world.SetResource(w, netcore.OutboundQueue{})
	world.SetResource(w, netcore.TransportResource{T: tr})
	return w, tr
}

func feedInputs(t *testing.T, w *world.World, tick uint64, peers []netcore.PlayerID) {
	t.Helper()
	bufP, ok := world.Resource[*netcore.InputBuffer](w)
	if !ok {
		t.Fatal("InputBuffer resource missing")
	}
	buf := *bufP
	for _, p := range peers {
		buf.RecordInput(tick, p, nil)
	}
}

// ─── LockstepScheduler ───────────────────────────────────────────────────────

func TestLockstepSchedulerDefaults(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	if ls.cfg.InputDelay != DefaultInputDelay {
		t.Errorf("InputDelay = %d, want %d", ls.cfg.InputDelay, DefaultInputDelay)
	}
	if ls.cfg.ChecksumInterval != DefaultChecksumInterval {
		t.Errorf("ChecksumInterval = %d, want %d", ls.cfg.ChecksumInterval, DefaultChecksumInterval)
	}
	if ls.cfg.InputTimeout != DefaultInputTimeout {
		t.Errorf("InputTimeout = %v, want %v", ls.cfg.InputTimeout, DefaultInputTimeout)
	}
}

func TestLockstepSchedulerCurrentTick(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}})
	if ls.CurrentTick() != 0 {
		t.Errorf("CurrentTick() = %d, want 0", ls.CurrentTick())
	}
}

func TestLockstepSchedulerHaltedInitially(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	if ls.Halted() {
		t.Error("Halted() should be false initially")
	}
}

func TestLockstepSchedulerStepNoResources(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}})
	w := world.NewWorld()
	if ls.Step(w) {
		t.Error("Step with missing resources should return false")
	}
}

func TestLockstepSchedulerStepHalted(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}})
	ls.halted = true
	w, _ := buildLockstepWorld(t)
	if ls.Step(w) {
		t.Error("Step when halted should return false")
	}
}

func TestLockstepSchedulerStepWaitsForPeers(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:      []netcore.PlayerID{1, 2},
		InputDelay: 2,
	})
	w, _ := buildLockstepWorld(t)
	// Peer 2's input for tick 2 is absent.
	feedInputs(t, w, 2, []netcore.PlayerID{1})
	if ls.Step(w) {
		t.Error("Step should return false when not all peers have input")
	}
	if ls.CurrentTick() != 0 {
		t.Errorf("CurrentTick() = %d, want 0 (no advance)", ls.CurrentTick())
	}
}

func TestLockstepSchedulerStepAdvancesWhenAllReady(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:            []netcore.PlayerID{1, 2},
		InputDelay:       2,
		ChecksumInterval: 100,
	})
	w, _ := buildLockstepWorld(t)
	feedInputs(t, w, 2, []netcore.PlayerID{1, 2})
	if !ls.Step(w) {
		t.Error("Step should return true when all peers have input")
	}
	if ls.CurrentTick() != 1 {
		t.Errorf("CurrentTick() = %d, want 1", ls.CurrentTick())
	}
}

func TestLockstepSchedulerStepChecksumBroadcast(t *testing.T) {
	t.Parallel()
	// checksumInterval=1 so every tick triggers a broadcast.
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:            []netcore.PlayerID{1},
		InputDelay:       1,
		ChecksumInterval: 1,
	})
	w, _ := buildLockstepWorld(t)
	feedInputs(t, w, 1, []netcore.PlayerID{1})
	if !ls.Step(w) {
		t.Fatal("Step should advance")
	}
	// Checksum broadcast should be in OutboundQueue.
	oqP, _ := world.Resource[netcore.OutboundQueue](w)
	if len(oqP.Messages) == 0 {
		t.Error("checksum broadcast should be enqueued in OutboundQueue")
	}
	// Decode and verify structure.
	tick, _, ok := DecodeChecksumPacket(oqP.Messages[0].Payload)
	if !ok {
		t.Fatal("DecodeChecksumPacket failed")
	}
	if tick != 1 {
		t.Errorf("broadcast tick = %d, want 1", tick)
	}
}

func TestLockstepSchedulerReceiveChecksumMatch(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:            []netcore.PlayerID{1},
		InputDelay:       1,
		ChecksumInterval: 100,
	})
	w, _ := buildLockstepWorld(t)
	feedInputs(t, w, 1, []netcore.PlayerID{1})
	ls.Step(w)

	// Get the local snapshot checksum.
	snapsP, _ := world.Resource[*netcore.SnapshotManager](w)
	snaps := *snapsP
	h, _ := snaps.Get(1)

	// Peer sends the same checksum → no desync.
	ls.ReceiveChecksum(w, 1, 1, h.Checksum)
	if ls.Halted() {
		t.Error("matching checksum should not halt scheduler")
	}
}

func TestLockstepSchedulerReceiveChecksumMismatchHalts(t *testing.T) {
	t.Parallel()
	var detected *DesyncDetected
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:            []netcore.PlayerID{1},
		InputDelay:       1,
		ChecksumInterval: 100,
	})
	ls.OnDesync = func(d DesyncDetected) { detected = &d }
	w, _ := buildLockstepWorld(t)
	feedInputs(t, w, 1, []netcore.PlayerID{1})
	ls.Step(w)

	// Peer sends a wrong checksum.
	ls.ReceiveChecksum(w, 1, 1, 0xDEADBEEF)
	if !ls.Halted() {
		t.Error("mismatched checksum should halt scheduler")
	}
	if detected == nil {
		t.Error("OnDesync should be called on mismatch")
	} else if detected.Tick != 1 || detected.Peer != 1 {
		t.Errorf("DesyncDetected = %+v, want Tick=1 Peer=1", detected)
	}
}

func TestLockstepSchedulerReceiveChecksumHaltedIgnored(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}})
	ls.halted = true
	w, _ := buildLockstepWorld(t)
	// Should not panic or call OnDesync.
	calls := 0
	ls.OnDesync = func(_ DesyncDetected) { calls++ }
	ls.ReceiveChecksum(w, 1, 1, 0xFF)
	if calls != 0 {
		t.Error("ReceiveChecksum when halted should be a no-op")
	}
}

func TestLockstepSchedulerReceiveChecksumMissingSnapshot(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}})
	w, _ := buildLockstepWorld(t)
	// Snapshot for tick 99 doesn't exist; should not panic or halt.
	ls.ReceiveChecksum(w, 1, 99, 0xFF)
	if ls.Halted() {
		t.Error("missing snapshot should not cause a halt")
	}
}

func TestLockstepSchedulerReceiveChecksumMissingResource(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}})
	w := world.NewWorld() // no SnapshotManager
	ls.ReceiveChecksum(w, 1, 1, 0xFF)
	if ls.Halted() {
		t.Error("missing SnapshotManager resource should not halt")
	}
}

func TestLockstepSchedulerStallSignal(t *testing.T) {
	t.Parallel()
	stallCh := make(chan WaitingForPlayers, 1)
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:        []netcore.PlayerID{1, 2},
		InputDelay:   1,
		InputTimeout: 10 * time.Second,
	})
	ls.OnStall = func(wp WaitingForPlayers) {
		select {
		case stallCh <- wp:
		default:
		}
	}
	w, _ := buildLockstepWorld(t)

	// Only peer 1 submitted input; stall clock starts on first failed Step.
	feedInputs(t, w, 1, []netcore.PlayerID{1})
	ls.Step(w) // sets stallSince

	// Manually push stallSince back to trigger the signal on next Step.
	ls.stallSince = time.Now().Add(-stallThreshold - time.Millisecond)
	ls.Step(w)

	select {
	case wp := <-stallCh:
		if wp.Tick == 0 {
			t.Error("stall signal should carry a non-zero tick")
		}
	default:
		t.Error("OnStall should have been called")
	}
}

func TestLockstepSchedulerTimeoutDisconnects(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{
		Peers:        []netcore.PlayerID{1, 2},
		InputDelay:   1,
		InputTimeout: time.Millisecond,
	})
	w, tr := buildLockstepWorld(t)

	// Only peer 1 has input; peer 2 is absent.
	feedInputs(t, w, 1, []netcore.PlayerID{1})
	ls.Step(w) // start stall clock

	// Exceed input timeout.
	ls.stallSince = time.Now().Add(-2 * time.Millisecond)
	ls.Step(w)

	// Peer 2 (ConnectionID=2) should be disconnected.
	disconnected := false
	for _, id := range tr.disconnected {
		if id == netcore.ConnectionID(2) {
			disconnected = true
		}
	}
	if !disconnected {
		t.Error("peer 2 should be disconnected after input timeout")
	}
}

// ─── LocalInputSystem ────────────────────────────────────────────────────────

func TestLocalInputSystemRun(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)
	sys := NewLocalInputSystem(5, 1)
	called := false
	sys.CaptureInput = func() []byte {
		called = true
		return []byte{42}
	}
	sys.Run(w)

	if !called {
		t.Error("CaptureInput was not called")
	}
	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	// RollbackCoordinator.Current()=0, so targetTick = 0+1=1.
	in, ok := buf.GetInput(1, 5)
	if !ok {
		t.Fatal("input for player 5 tick 1 not recorded")
	}
	if string(in.Data) != string([]byte{42}) {
		t.Errorf("input data = %v, want [42]", in.Data)
	}
}

func TestLocalInputSystemBroadcasts(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)
	sys := NewLocalInputSystem(3, 1)
	sys.Run(w)

	oqP, _ := world.Resource[netcore.OutboundQueue](w)
	if len(oqP.Messages) == 0 {
		t.Error("LocalInputSystem should enqueue a broadcast")
	}
	if oqP.Messages[0].Target != 0 {
		t.Errorf("Target = %d, want 0 (broadcast)", oqP.Messages[0].Target)
	}
}

func TestLocalInputSystemNoResources(t *testing.T) {
	t.Parallel()
	sys := NewLocalInputSystem(1, 0)
	w := world.NewWorld()
	sys.Run(w) // should not panic
}

func TestLocalInputSystemNilCapture(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)
	sys := NewLocalInputSystem(1, 1)
	// CaptureInput is nil; should record a nil/empty input without panic.
	sys.Run(w)
	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	in, ok := buf.GetInput(1, 1)
	if !ok {
		t.Fatal("input should be recorded even with nil CaptureInput")
	}
	if len(in.Data) != 0 {
		t.Errorf("data = %v, want empty", in.Data)
	}
}

// ─── RemoteInputSystem ───────────────────────────────────────────────────────

func TestRemoteInputSystemRecordsInput(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)

	// Inject a packet into InboundQueue.
	payload := encodeInputPacket(7, 42, []byte("left"))
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelEvents, Payload: payload},
	}

	sys := RemoteInputSystem{}
	sys.Run(w)

	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	in, ok := buf.GetInput(42, 7)
	if !ok {
		t.Fatal("remote input for player 7 tick 42 not recorded")
	}
	if string(in.Data) != "left" {
		t.Errorf("data = %q, want %q", in.Data, "left")
	}
}

func TestRemoteInputSystemIgnoresOtherChannels(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)

	// Snapshot channel packet should be ignored by RemoteInputSystem.
	payload := encodeInputPacket(1, 1, []byte("data"))
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: payload},
	}

	sys := RemoteInputSystem{}
	sys.Run(w)

	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	if _, ok := buf.GetInput(1, 1); ok {
		t.Error("snapshot channel packets should be ignored by RemoteInputSystem")
	}
}

func TestRemoteInputSystemShortPayloadIgnored(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)

	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelEvents, Payload: []byte{0, 1}}, // too short
	}
	sys := RemoteInputSystem{}
	sys.Run(w) // should not panic
}

func TestRemoteInputSystemNoResources(t *testing.T) {
	t.Parallel()
	sys := RemoteInputSystem{}
	w := world.NewWorld()
	sys.Run(w) // should not panic
}

// ─── packet encode/decode ────────────────────────────────────────────────────

func TestInputPacketRoundtrip(t *testing.T) {
	t.Parallel()
	payload := encodeInputPacket(3, 77, []byte("jump"))
	p, tick, data, ok := decodeInputPacket(payload)
	if !ok {
		t.Fatal("decodeInputPacket: want ok=true")
	}
	if p != 3 || tick != 77 || string(data) != "jump" {
		t.Errorf("got player=%d tick=%d data=%q, want 3 77 jump", p, tick, data)
	}
}

func TestInputPacketShort(t *testing.T) {
	t.Parallel()
	_, _, _, ok := decodeInputPacket([]byte{0, 1, 2})
	if ok {
		t.Error("decodeInputPacket with 3 bytes: want ok=false")
	}
}

func TestChecksumPacketRoundtrip(t *testing.T) {
	t.Parallel()
	payload := encodeChecksumPacket(55, 0xCAFEBABE)
	tick, cs, ok := DecodeChecksumPacket(payload)
	if !ok {
		t.Fatal("DecodeChecksumPacket: want ok=true")
	}
	if tick != 55 || cs != 0xCAFEBABE {
		t.Errorf("got tick=%d cs=0x%X, want 55 0xCAFEBABE", tick, cs)
	}
}

func TestChecksumPacketShort(t *testing.T) {
	t.Parallel()
	_, _, ok := DecodeChecksumPacket([]byte{0, 1})
	if ok {
		t.Error("DecodeChecksumPacket with 2 bytes: want ok=false")
	}
}
