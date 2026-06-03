package oop

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	pkgplugin "github.com/neuengine/neu/pkg/plugin"
	"github.com/neuengine/neu/pkg/protocol"
)

// fakePluginEnv is read by the test binary when re-invoked as an OOP plugin.
const fakePluginEnv = "OOP_TEST_FAKE_PLUGIN"

// TestMain routes subprocess invocations to the fake-plugin logic.
// Tests that use real subprocesses re-exec os.Args[0] (the test binary) with
// OOP_TEST_FAKE_PLUGIN set; TestMain detects that and runs the plugin logic
// instead of the test suite.
func TestMain(m *testing.M) {
	switch os.Getenv(fakePluginEnv) {
	case "normal":
		runFakePlugin(true)
		os.Exit(0)
	case "crash":
		// Respond to handshake then exit with a non-zero code.
		runFakePlugin(false)
		os.Exit(2)
	}
	os.Exit(m.Run())
}

// runFakePlugin is the fake plugin binary logic run in the subprocess.
// If respondLifecycle is true it echoes Done for every lifecycle message.
func runFakePlugin(respondLifecycle bool) {
	scanner := protocol.NewScanner(os.Stdin)
	// 1. Consume PluginInit.
	if _, _, ok, err := scanner.Next(); !ok || err != nil {
		return
	}
	// 2. Send PluginHello.
	_ = protocol.Encode(os.Stdout, protocol.PluginHello{
		Type:            protocol.KindPluginHello,
		ID:              "com.test.spawn",
		Version:         "0.1.0",
		ProtocolVersion: "1",
	})
	if !respondLifecycle {
		return
	}
	// 3. Echo PluginLifecycleDone for every PluginLifecycle received.
	for {
		msg, _, ok, err := scanner.Next()
		if !ok || err != nil {
			return
		}
		if lc, ok := msg.(protocol.PluginLifecycle); ok {
			_ = protocol.Encode(os.Stdout, protocol.PluginLifecycleDone{
				Type:  protocol.KindPluginLifecycleDone,
				Phase: lc.Phase,
			})
		}
	}
}

// --- fakeBuilder (minimal appface.Builder for tests) -------------------------

type fakeBuilder struct{ w *world.World }

func newFakeBuilder() *fakeBuilder { return &fakeBuilder{w: world.NewWorld()} }

func (b *fakeBuilder) World() *world.World                                        { return b.w }
func (b *fakeBuilder) AddSystem(_ string, _ scheduler.System) appface.Builder     { return b }
func (b *fakeBuilder) AddSystems(_ string, _ ...scheduler.System) appface.Builder { return b }
func (b *fakeBuilder) SetResource(any) appface.Builder                            { return b }
func (b *fakeBuilder) InitResource(any) appface.Builder                           { return b }
func (b *fakeBuilder) AddPlugin(appface.Plugin) appface.Builder                   { return b }
func (b *fakeBuilder) AddPlugins(appface.PluginGroup) appface.Builder             { return b }

var _ appface.Builder = (*fakeBuilder)(nil)

// --- fakePeer (simulates the OOP plugin subprocess over in-process pipes) ----

type fakePeer struct {
	in  *json.Decoder
	out *io.PipeWriter
}

func newFakePeer(id string) (*Supervisor, *fakePeer) {
	engineStdinR, engineStdinW := io.Pipe()
	engineStdoutR, engineStdoutW := io.Pipe()

	c := newConn(engineStdinW, engineStdoutR)
	sup := newSupervisorFromConn(id, c)

	return sup, &fakePeer{
		in:  json.NewDecoder(engineStdinR),
		out: engineStdoutW,
	}
}

func (p *fakePeer) recv(v any) error { return p.in.Decode(v) }
func (p *fakePeer) send(msg any) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(p.out, "%s\n", b)
	return err
}
func (p *fakePeer) closeOutput() { _ = p.out.Close() }

// spawnForTest re-execs the current test binary with OOP_TEST_FAKE_PLUGIN=mode
// so spawnFromCmd (and therefore Spawn code paths) are exercised.
func spawnForTest(t *testing.T, mode string, init protocol.PluginInit) *Supervisor {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=SKIP_ALL_TESTS_THIS_IS_A_PLUGIN")
	cmd.Dir = t.TempDir()
	cmd.Env = append(os.Environ(), fakePluginEnv+"="+mode)

	sup, err := spawnFromCmd("com.test.spawn", cmd, init)
	if err != nil {
		t.Fatalf("spawnForTest(%s): %v", mode, err)
	}
	t.Cleanup(func() { sup.Close() })
	return sup
}

// peerEchoDone starts a goroutine that echoes PluginLifecycleDone for every
// received lifecycle message until the peer's input returns an error.
func peerEchoDone(peer *fakePeer) {
	go func() {
		for {
			var raw struct {
				Phase string `json:"phase"`
			}
			if err := peer.recv(&raw); err != nil {
				return
			}
			_ = peer.send(protocol.PluginLifecycleDone{
				Type:  protocol.KindPluginLifecycleDone,
				Phase: protocol.LifecyclePhase(raw.Phase),
			})
		}
	}()
}

// --- Spawn integration tests (uses real subprocesses via test binary) --------

func TestSpawn_NormalLifecycle(t *testing.T) {
	ctx := context.Background()
	init := protocol.PluginInit{EngineVersion: "0.1.0"}
	sup := spawnForTest(t, "normal", init)

	if sup.IsFailed() {
		t.Fatal("supervisor should not be failed after successful Spawn")
	}
	if err := sup.DriveLifecycle(ctx, protocol.PhaseBuild); err != nil {
		t.Fatalf("DriveLifecycle(Build): %v", err)
	}
	if err := sup.DriveLifecycle(ctx, protocol.PhaseReady); err != nil {
		t.Fatalf("DriveLifecycle(Ready): %v", err)
	}
}

func TestSpawn_SubprocessCrash(t *testing.T) {
	init := protocol.PluginInit{EngineVersion: "0.1.0"}
	sup := spawnForTest(t, "crash", init)

	// monitorProcess will detect the process exit and call MarkFailed.
	select {
	case <-sup.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("supervisor Done channel not closed after subprocess exit")
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be failed after subprocess crash (INV-8)")
	}
}

// --- Supervisor handshake tests (in-process pipes) ---------------------------

func TestSupervisor_HandshakeSuccess(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")

	go func() {
		var raw struct{ Type string }
		if err := peer.recv(&raw); err != nil {
			return
		}
		_ = peer.send(protocol.PluginHello{
			Type:            protocol.KindPluginHello,
			ID:              "com.test.plugin",
			Version:         "0.1.0",
			ProtocolVersion: "1",
		})
	}()

	if err := sup.handshake(protocol.PluginInit{EngineVersion: "0.1.0"}); err != nil {
		t.Fatalf("handshake: %v", err)
	}
	if sup.IsFailed() {
		t.Fatal("supervisor should not be failed after successful handshake")
	}
}

func TestSupervisor_HandshakeWrongKind(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")

	go func() {
		var raw struct{ Type string }
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginLifecycleDone{
			Type:  protocol.KindPluginLifecycleDone,
			Phase: protocol.PhaseBuild,
		})
	}()

	if err := sup.handshake(protocol.PluginInit{EngineVersion: "0.1.0"}); err == nil {
		t.Fatal("expected handshake error for wrong reply kind")
	}
}

func TestSupervisor_HandshakeTimeout(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")
	orig := handshakeTimeout
	handshakeTimeout = 30 * time.Millisecond
	defer func() { handshakeTimeout = orig }()

	go func() {
		var raw struct{ Type string }
		_ = peer.recv(&raw)
		// Read PluginInit but never respond → handshake times out.
	}()

	if err := sup.handshake(protocol.PluginInit{EngineVersion: "0.1.0"}); err == nil {
		t.Fatal("expected handshake timeout error")
	}
}

func TestSupervisor_HandshakeSendError(t *testing.T) {
	sup, _ := newFakePeer("com.test.plugin")
	if pw, ok := sup.c.w.(*io.PipeWriter); ok {
		_ = pw.Close()
	}

	if err := sup.handshake(protocol.PluginInit{EngineVersion: "0.1.0"}); err == nil {
		t.Fatal("expected handshake error when send fails")
	}
}

// --- DriveLifecycle tests (in-process pipes) ---------------------------------

func TestSupervisor_DriveLifecycle_Success(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")
	peerEchoDone(peer)

	for _, phase := range []protocol.LifecyclePhase{
		protocol.PhaseBuild, protocol.PhaseReady, protocol.PhaseFinish, protocol.PhaseCleanup,
	} {
		if err := sup.DriveLifecycle(context.Background(), phase); err != nil {
			t.Fatalf("DriveLifecycle(%s): %v", phase, err)
		}
	}
	if sup.IsFailed() {
		t.Fatal("supervisor should not be failed after successful lifecycle")
	}
}

func TestSupervisor_DriveLifecycle_PluginError(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginError{
			Type:    protocol.KindPluginError,
			Phase:   protocol.PhaseBuild,
			Message: "build failed",
			Code:    42,
		})
	}()

	if err := sup.DriveLifecycle(context.Background(), protocol.PhaseBuild); err == nil {
		t.Fatal("expected error from PluginError reply")
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be failed after plugin error")
	}
}

func TestSupervisor_DriveLifecycle_PhaseMismatch(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginLifecycleDone{
			Type:  protocol.KindPluginLifecycleDone,
			Phase: protocol.PhaseReady, // wrong
		})
	}()

	if err := sup.DriveLifecycle(context.Background(), protocol.PhaseBuild); err == nil {
		t.Fatal("expected error on phase mismatch")
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be failed on phase mismatch")
	}
}

func TestSupervisor_DriveLifecycle_ContextCancelled(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
	}()

	if err := sup.DriveLifecycle(ctx, protocol.PhaseBuild); err == nil {
		t.Fatal("expected error when context is cancelled")
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be failed on context cancellation")
	}
}

func TestSupervisor_DriveLifecycle_AlreadyFailed(t *testing.T) {
	sup, _ := newFakePeer("com.test.plugin")
	sup.MarkFailed()

	if err := sup.DriveLifecycle(context.Background(), protocol.PhaseBuild); err != ErrPluginFailed {
		t.Fatalf("expected ErrPluginFailed, got %v", err)
	}
}

func TestSupervisor_DriveLifecycle_SendError(t *testing.T) {
	sup, _ := newFakePeer("com.test.plugin")
	if pw, ok := sup.c.w.(*io.PipeWriter); ok {
		_ = pw.Close()
	}

	if err := sup.DriveLifecycle(context.Background(), protocol.PhaseBuild); err == nil {
		t.Fatal("expected error when send fails")
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be failed after send error")
	}
}

func TestSupervisor_DriveLifecycle_TransportEOF(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		peer.closeOutput()
	}()

	if err := sup.DriveLifecycle(context.Background(), protocol.PhaseBuild); err == nil {
		t.Fatal("expected error on transport close")
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be failed on transport error")
	}
}

func TestSupervisor_DriveLifecycle_UnexpectedKind(t *testing.T) {
	sup, peer := newFakePeer("com.test.plugin")

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginInit{Type: protocol.KindPluginInit, EngineVersion: "0.1.0"})
	}()

	if err := sup.DriveLifecycle(context.Background(), protocol.PhaseBuild); err == nil {
		t.Fatal("expected error on unexpected message kind")
	}
}

func TestSupervisor_MarkFailed_Idempotent(t *testing.T) {
	sup, _ := newFakePeer("com.test.plugin")
	sup.MarkFailed()
	sup.MarkFailed() // must not panic
	if !sup.IsFailed() {
		t.Fatal("supervisor must be failed after MarkFailed")
	}
}

func TestSupervisor_Done(t *testing.T) {
	sup, _ := newFakePeer("com.test.plugin")
	select {
	case <-sup.Done():
		t.Fatal("done should not be closed before MarkFailed")
	default:
	}
	sup.MarkFailed()
	select {
	case <-sup.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("done channel not closed after MarkFailed")
	}
}

func TestSupervisor_MonitorProcess_NilCmd(t *testing.T) {
	sup, _ := newFakePeer("com.test.plugin")
	sup.monitorProcess() // nil cmd → returns immediately, no failure
	if sup.IsFailed() {
		t.Fatal("nil-cmd monitorProcess must not mark supervisor failed")
	}
}

// --- ProxyPlugin tests -------------------------------------------------------

type mockStateRecorder struct{ states []pkgplugin.State }

func (r *mockStateRecorder) set(id pkgplugin.PluginID, s pkgplugin.State) {
	r.states = append(r.states, s)
}

func (r *mockStateRecorder) last() pkgplugin.State {
	if len(r.states) == 0 {
		return 0
	}
	return r.states[len(r.states)-1]
}

func TestProxyPlugin_FullLifecycle(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, peer := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)
	peerEchoDone(peer)

	b := newFakeBuilder()
	proxy.Build(b)
	if rec.last() != pkgplugin.StateActive {
		t.Fatalf("after Build expected StateActive, got %v", rec.last())
	}

	proxy.Ready(b)
	proxy.Finish(b)
	proxy.Cleanup(b)

	if rec.last() != pkgplugin.StateDisabled {
		t.Fatalf("after Cleanup expected StateDisabled, got %v", rec.last())
	}
	if !sup.IsFailed() {
		t.Fatal("supervisor should be closed/failed after Cleanup")
	}
}

func TestProxyPlugin_Build_Failure(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, peer := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginError{
			Type: protocol.KindPluginError, Phase: protocol.PhaseBuild, Message: "oops",
		})
	}()

	proxy.Build(newFakeBuilder())
	if rec.last() != pkgplugin.StateFailed {
		t.Fatalf("after failed Build expected StateFailed, got %v", rec.last())
	}
}

func TestProxyPlugin_Ready_SkippedWhenFailed(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, _ := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)
	sup.MarkFailed()

	proxy.Ready(newFakeBuilder())
	if len(rec.states) != 0 {
		t.Fatalf("Ready must be a no-op when supervisor is failed, got %v", rec.states)
	}
}

func TestProxyPlugin_Finish_SkippedWhenFailed(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, _ := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)
	sup.MarkFailed()

	proxy.Finish(newFakeBuilder())
	if len(rec.states) != 0 {
		t.Fatalf("Finish must be a no-op when supervisor is failed, got %v", rec.states)
	}
}

func TestProxyPlugin_Cleanup_SetsDisabledEvenWhenFailed(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, _ := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)
	sup.MarkFailed()

	proxy.Cleanup(newFakeBuilder())
	if rec.last() != pkgplugin.StateDisabled {
		t.Fatalf("Cleanup must still set Disabled when failed, got %v", rec.last())
	}
}

func TestProxyPlugin_Ready_Failure(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, peer := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginError{
			Type: protocol.KindPluginError, Phase: protocol.PhaseReady, Message: "ready error",
		})
	}()

	proxy.Ready(newFakeBuilder())
	if rec.last() != pkgplugin.StateFailed {
		t.Fatalf("failed Ready should set StateFailed, got %v", rec.last())
	}
}

func TestProxyPlugin_Finish_Failure(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, peer := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		_ = peer.send(protocol.PluginError{
			Type: protocol.KindPluginError, Phase: protocol.PhaseFinish, Message: "finish error",
		})
	}()

	proxy.Finish(newFakeBuilder())
	if rec.last() != pkgplugin.StateFailed {
		t.Fatalf("failed Finish should set StateFailed, got %v", rec.last())
	}
}

func TestProxyPlugin_Cleanup_DriveError(t *testing.T) {
	id := pkgplugin.PluginID("com.test.proxy")
	sup, peer := newFakePeer(string(id))
	rec := &mockStateRecorder{}
	proxy := NewProxyPlugin(id, sup, rec.set)

	go func() {
		var raw struct {
			Phase string `json:"phase"`
		}
		_ = peer.recv(&raw)
		peer.closeOutput()
	}()

	proxy.Cleanup(newFakeBuilder())
	if rec.last() != pkgplugin.StateDisabled {
		t.Fatalf("Cleanup with error: expected StateDisabled, got %v", rec.last())
	}
}

// compile-time interface assertions (also in proxyplugin.go)
var _ appface.Plugin = (*ProxyPlugin)(nil)
var _ appface.FullPlugin = (*ProxyPlugin)(nil)
