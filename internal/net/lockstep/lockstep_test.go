package lockstep

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/app/appface"
	"github.com/neuengine/neu/pkg/scene"
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

// ─── LockstepScheduler.Run ───────────────────────────────────────────────────

func TestLockstepSchedulerRun(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{1}, InputDelay: 1})
	w, _ := buildLockstepWorld(t)
	feedInputs(t, w, 1, []netcore.PlayerID{1})
	ls.Run(w)
	if ls.CurrentTick() != 1 {
		t.Errorf("CurrentTick() = %d, want 1 after Run", ls.CurrentTick())
	}
}

// ─── DesyncReceiveSystem ──────────────────────────────────────────────────────

func TestDesyncReceiveSystemNoResources(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewDesyncReceiveSystem(ls)
	w := world.NewWorld()
	sys.Run(w) // no InboundQueue resource → early return, no panic
}

func TestDesyncReceiveSystemIgnoresNonEventChannel(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewDesyncReceiveSystem(ls)
	w, _ := buildLockstepWorld(t)

	payload := encodeChecksumPacket(1, 0xDEAD)
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: payload},
	}
	sys.Run(w)
	if ls.Halted() {
		t.Error("packet on non-events channel should not cause halt")
	}
}

func TestDesyncReceiveSystemHaltOnMismatch(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	desyncCh := make(chan DesyncDetected, 1)
	ls.OnDesync = func(d DesyncDetected) { desyncCh <- d }
	sys := NewDesyncReceiveSystem(ls)
	w, _ := buildLockstepWorld(t)

	// Record a snapshot at tick 3 so ReceiveChecksum has something to compare.
	snapsP, _ := world.Resource[*netcore.SnapshotManager](w)
	snaps := *snapsP
	reader := scene.NewWorldAdapter(w)
	snaps.TakeSnapshot(reader, 3)

	// Send a checksum that differs from the local snapshot → mismatch.
	payload := encodeChecksumPacket(3, 0xBADCAFE)
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelEvents, Connection: netcore.ConnectionID(2), Payload: payload},
	}
	sys.Run(w)

	if !ls.Halted() {
		t.Error("scheduler should halt on checksum mismatch")
	}
	select {
	case d := <-desyncCh:
		if d.Tick != 3 {
			t.Errorf("DesyncDetected.Tick = %d, want 3", d.Tick)
		}
		if d.Peer != 2 {
			t.Errorf("DesyncDetected.Peer = %d, want 2", d.Peer)
		}
	default:
		t.Error("OnDesync should have been called")
	}
}

func TestDesyncReceiveSystemNoHaltOnMatch(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewDesyncReceiveSystem(ls)
	w, _ := buildLockstepWorld(t)

	// Record a real snapshot and use its exact checksum.
	snapsP, _ := world.Resource[*netcore.SnapshotManager](w)
	snaps := *snapsP
	reader := scene.NewWorldAdapter(w)
	h := snaps.TakeSnapshot(reader, 7)

	payload := encodeChecksumPacket(7, h.Checksum)
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelEvents, Payload: payload},
	}
	sys.Run(w)

	if ls.Halted() {
		t.Error("scheduler should not halt when checksums match")
	}
}

// ─── SpeculativeExecutor ─────────────────────────────────────────────────────

func TestSpeculativeExecutorDefaultMaxSpec(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: true})
	if ex.config.MaxSpeculative != defaultMaxSpeculative {
		t.Errorf("MaxSpeculative = %d, want %d", ex.config.MaxSpeculative, defaultMaxSpeculative)
	}
}

func TestSpeculativeExecutorDisabledNoOp(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{2}, InputDelay: 1})
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: false})
	w, _ := buildLockstepWorld(t)
	ex.Run(w)
	if ex.specCount != 0 || len(ex.pending) != 0 {
		t.Error("disabled executor should leave specCount=0 and pending empty")
	}
}

func TestSpeculativeExecutorHaltedNoOp(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{2}, InputDelay: 1})
	ls.halted = true
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: true})
	w, _ := buildLockstepWorld(t)
	ex.Run(w)
	if len(ex.pending) != 0 {
		t.Error("halted scheduler: executor should not predict")
	}
}

func TestSpeculativeExecutorNoResources(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{2}, InputDelay: 1})
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: true})
	w := world.NewWorld()
	ex.Run(w) // no resources → early return, no panic
}

func TestSpeculativeExecutorPredictsInput(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{2}, InputDelay: 1})
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: true})
	w, _ := buildLockstepWorld(t)

	ex.Run(w)

	// target = ls.currentTick(0) + InputDelay(1) = 1
	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	if _, ok := buf.GetInput(1, 2); !ok {
		t.Error("speculative executor should predict input for missing peer")
	}
	if ex.specCount != 1 {
		t.Errorf("specCount = %d, want 1", ex.specCount)
	}
	if len(ex.pending) != 1 {
		t.Errorf("len(pending) = %d, want 1", len(ex.pending))
	}
}

func TestSpeculativeExecutorConfirmsCorrect(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{2}, InputDelay: 1})
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: true})
	w, _ := buildLockstepWorld(t)

	// First run: predict nil data (PredictInput with no history returns nil Data).
	ex.Run(w)
	if ex.specCount != 1 {
		t.Fatalf("specCount = %d, want 1 after first run", ex.specCount)
	}

	// Real input arrives matching the prediction (nil bytes → same checksum).
	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	buf.RecordInput(1, 2, nil)

	// Second run: confirm, pending cleared, no rollback.
	ex.Run(w)
	if ex.specCount != 0 {
		t.Errorf("specCount = %d after correct confirm, want 0", ex.specCount)
	}
	if len(ex.pending) != 0 {
		t.Errorf("len(pending) = %d after correct confirm, want 0", len(ex.pending))
	}
}

func TestSpeculativeExecutorRollbackOnMismatch(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{Peers: []netcore.PlayerID{2}, InputDelay: 1})
	ex := NewSpeculativeExecutor(ls, SpeculativeConfig{Enabled: true})
	w, _ := buildLockstepWorld(t)

	// First run: predict nil data for player 2 at tick 1.
	ex.Run(w)
	if ex.specCount != 1 {
		t.Fatalf("specCount = %d, want 1", ex.specCount)
	}

	// Real input arrives with conflicting data → mismatch.
	bufP, _ := world.Resource[*netcore.InputBuffer](w)
	buf := *bufP
	buf.RecordInput(1, 2, []byte{0xFF})

	// Second run: misprediction detected, pending and specCount cleared.
	ex.Run(w)
	if ex.specCount != 0 {
		t.Errorf("specCount = %d after mismatch, want 0", ex.specCount)
	}
	if len(ex.pending) != 0 {
		t.Errorf("len(pending) = %d after mismatch, want 0", len(ex.pending))
	}
}

// ─── LatejoinSystem ──────────────────────────────────────────────────────────

// trackingTransport extends mockTransport to record per-connection sends and
// expose a configurable connection list.
type trackingTransport struct {
	mockTransport
	conns []netcore.ConnectionID
	sent  []trackingSent
}

type trackingSent struct {
	conn    netcore.ConnectionID
	channel netcore.ChannelID
	payload []byte
}

func (m *trackingTransport) Connections() []netcore.ConnectionID { return m.conns }
func (m *trackingTransport) Send(id netcore.ConnectionID, ch netcore.ChannelID, p []byte) error {
	m.sent = append(m.sent, trackingSent{conn: id, channel: ch, payload: p})
	return nil
}

func TestLatejoinPacketRoundtrip(t *testing.T) {
	t.Parallel()
	data := []byte("snapshot-bytes")
	payload := encodeLateJoinPacket(42, data)
	tick, got, ok := decodeLateJoinPacket(payload)
	if !ok {
		t.Fatal("decodeLateJoinPacket: want ok=true")
	}
	if tick != 42 {
		t.Errorf("tick = %d, want 42", tick)
	}
	if string(got) != string(data) {
		t.Errorf("data = %q, want %q", got, data)
	}
}

func TestLatejoinPacketShortPayload(t *testing.T) {
	t.Parallel()
	_, _, ok := decodeLateJoinPacket([]byte{0, 1, 2})
	if ok {
		t.Error("short payload: want ok=false")
	}
}

func TestLatejoinPacketWrongMagic(t *testing.T) {
	t.Parallel()
	payload := make([]byte, 12)
	copy(payload, []byte{'X', 'X', 'X', 'X'})
	_, _, ok := decodeLateJoinPacket(payload)
	if ok {
		t.Error("wrong magic bytes: want ok=false")
	}
}

func TestLatejoinSystemServerNoResources(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, true)
	w := world.NewWorld()
	sys.Run(w) // no TransportResource or SnapshotManager → early return, no panic
}

func TestLatejoinSystemServerSendsSnapshot(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, true)
	w, _ := buildLockstepWorld(t)

	// Record a snapshot at tick 0 (ls.currentTick).
	snapsP, _ := world.Resource[*netcore.SnapshotManager](w)
	snaps := *snapsP
	reader := scene.NewWorldAdapter(w)
	snaps.TakeSnapshot(reader, 0)

	tt := &trackingTransport{conns: []netcore.ConnectionID{7}}
	world.SetResource(w, netcore.TransportResource{T: tt})

	sys.Run(w)

	if len(tt.sent) != 1 {
		t.Fatalf("sent %d packets, want 1", len(tt.sent))
	}
	if tt.sent[0].conn != 7 {
		t.Errorf("sent to conn %d, want 7", tt.sent[0].conn)
	}
	if tt.sent[0].channel != netcore.ChannelSnapshot {
		t.Errorf("sent on channel %d, want ChannelSnapshot", tt.sent[0].channel)
	}
	tick, _, ok := decodeLateJoinPacket(tt.sent[0].payload)
	if !ok {
		t.Fatal("sent payload does not decode as latejoin packet")
	}
	if tick != 0 {
		t.Errorf("latejoin tick = %d, want 0", tick)
	}
}

func TestLatejoinSystemServerIgnoresKnownConn(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, true)
	w, _ := buildLockstepWorld(t)

	snapsP, _ := world.Resource[*netcore.SnapshotManager](w)
	snaps := *snapsP
	reader := scene.NewWorldAdapter(w)
	snaps.TakeSnapshot(reader, 0)

	tt := &trackingTransport{conns: []netcore.ConnectionID{5}}
	world.SetResource(w, netcore.TransportResource{T: tt})

	// First run: conn 5 is new → send snapshot.
	sys.Run(w)
	if len(tt.sent) != 1 {
		t.Fatalf("first run: sent %d packets, want 1", len(tt.sent))
	}
	// Second run: conn 5 is already known → no resend.
	sys.Run(w)
	if len(tt.sent) != 1 {
		t.Errorf("second run: sent %d total packets, want still 1 (known conn suppressed)", len(tt.sent))
	}
}

func TestLatejoinSystemClientNoResources(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, false)
	w := world.NewWorld()
	sys.Run(w) // no InboundQueue → early return, no panic
}

func TestLatejoinSystemClientAppliesSnapshot(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, false)
	w, _ := buildLockstepWorld(t)

	// Encode a latejoin packet with an empty but valid JSON snapshot.
	payload := encodeLateJoinPacket(10, []byte("[]"))
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: payload},
	}
	sys.Run(w)

	if ls.currentTick != 10 {
		t.Errorf("currentTick = %d, want 10 after latejoin", ls.currentTick)
	}
}

func TestLatejoinSystemClientIgnoresNonSnapshotChannel(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, false)
	w, _ := buildLockstepWorld(t)

	payload := encodeLateJoinPacket(5, []byte("[]"))
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelEvents, Payload: payload}, // wrong channel
	}
	sys.Run(w)

	if ls.currentTick != 0 {
		t.Errorf("currentTick = %d, want 0 (events-channel packet ignored)", ls.currentTick)
	}
}

func TestLatejoinSystemClientIgnoresBadMagic(t *testing.T) {
	t.Parallel()
	ls := NewLockstepScheduler(LockstepConfig{})
	sys := NewLatejoinSystem(ls, false)
	w, _ := buildLockstepWorld(t)

	bad := make([]byte, 12)
	copy(bad, []byte{'X', 'X', 'X', 'X'})
	iqP, _ := world.Resource[netcore.InboundQueue](w)
	iqP.Packets = []netcore.InboundPacket{
		{Channel: netcore.ChannelSnapshot, Payload: bad},
	}
	sys.Run(w)

	if ls.currentTick != 0 {
		t.Errorf("currentTick = %d, want 0 (bad magic ignored)", ls.currentTick)
	}
}

// ─── LockstepPlugin ──────────────────────────────────────────────────────────

type fakeLockstepBuilder struct {
	w       *world.World
	systems map[string][]string
}

func newFakeLockstepBuilder(w *world.World) *fakeLockstepBuilder {
	return &fakeLockstepBuilder{w: w, systems: make(map[string][]string)}
}

func (b *fakeLockstepBuilder) World() *world.World { return b.w }

func (b *fakeLockstepBuilder) AddSystem(schedule string, sys scheduler.System) appface.Builder {
	b.systems[schedule] = append(b.systems[schedule], sys.Name())
	return b
}

func (b *fakeLockstepBuilder) AddSystems(schedule string, sys ...scheduler.System) appface.Builder {
	for _, s := range sys {
		b.systems[schedule] = append(b.systems[schedule], s.Name())
	}
	return b
}

func (b *fakeLockstepBuilder) SetResource(_ any) appface.Builder          { return b }
func (b *fakeLockstepBuilder) InitResource(_ any) appface.Builder         { return b }
func (b *fakeLockstepBuilder) AddPlugin(_ appface.Plugin) appface.Builder { return b }
func (b *fakeLockstepBuilder) AddPlugins(_ appface.PluginGroup) appface.Builder {
	return b
}

func TestLockstepPluginBuild(t *testing.T) {
	t.Parallel()
	w, _ := buildLockstepWorld(t)
	fb := newFakeLockstepBuilder(w)

	p := LockstepPlugin{
		Config:      LockstepConfig{Peers: []netcore.PlayerID{1, 2}},
		LocalPlayer: 1,
		IsServer:    false,
		Speculative: SpeculativeConfig{Enabled: false},
	}
	p.Build(fb)

	wantSystems := map[string][]string{
		appface.PreUpdate:      {"lockstep.RemoteInputSystem", "lockstep.LatejoinSystem"},
		appface.FixedPreUpdate: {"lockstep.SpeculativeExecutor"},
		appface.FixedUpdate:    {"lockstep.LocalInputSystem", "lockstep.LockstepScheduler"},
		appface.PostUpdate:     {"lockstep.DesyncReceiveSystem"},
	}
	for sched, want := range wantSystems {
		got := fb.systems[sched]
		if len(got) != len(want) {
			t.Errorf("schedule %q: got %d systems %v, want %d %v", sched, len(got), got, len(want), want)
			continue
		}
		for i, name := range want {
			if got[i] != name {
				t.Errorf("schedule %q[%d]: got %q, want %q", sched, i, got[i], name)
			}
		}
	}
}
