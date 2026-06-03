// Package oop implements the out-of-process plugin loader: subprocess spawn
// (cwd-restricted, INV-8), pkg/protocol handshake, lifecycle driving, and
// failure isolation so a crashing subprocess never corrupts the host. T-6N04.
package oop

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"time"

	"github.com/neuengine/neu/pkg/protocol"
)

// handshakeTimeout is the maximum time the engine waits for PluginHello after
// sending PluginInit. A slow or broken plugin binary must not stall startup.
// Variable (not const) so tests can shrink it without sleeping.
var handshakeTimeout = 10 * time.Second

// ErrHandshakeFailed is returned by Spawn when the subprocess does not complete
// the PluginInit/PluginHello handshake within handshakeTimeout.
var ErrHandshakeFailed = errors.New("oop: handshake timeout or protocol error")

// ErrPluginFailed is returned by DriveLifecycle when the subprocess has already
// been marked Failed (crash, disconnect, or prior lifecycle error).
var ErrPluginFailed = errors.New("oop: plugin subprocess has failed")

// conn is the typed stdin/stdout wrapper the Supervisor uses for protocol I/O.
type conn struct {
	w       io.Writer
	scanner *protocol.Scanner
	mu      sync.Mutex
}

func newConn(w io.Writer, r io.Reader) *conn {
	return &conn{w: w, scanner: protocol.NewScanner(r)}
}

// send encodes msg and writes it to the subprocess stdin.
func (c *conn) send(msg any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return protocol.Encode(c.w, msg)
}

// receive reads and decodes the next protocol message from the subprocess stdout.
func (c *conn) receive() (any, protocol.Kind, error) {
	msg, kind, ok, err := c.scanner.Next()
	if err != nil {
		return nil, kind, err
	}
	if !ok {
		return nil, "", io.EOF
	}
	return msg, kind, nil
}

// Supervisor manages one out-of-process plugin subprocess. Obtain one via
// Spawn (real process) or newSupervisorFromConn (tests).
type Supervisor struct {
	id     string // plugin ID (for logging)
	c      *conn
	cmd    *exec.Cmd // nil in unit tests
	mu     sync.Mutex
	failed bool
	done   chan struct{} // closed when the subprocess exits or the supervisor is closed
	once   sync.Once     // guards done-channel close
}

// Spawn launches binary with its working directory restricted to workDir, sends
// the PluginInit handshake, and waits for PluginHello. id is the plugin ID
// (from the manifest) used only for logging. Returns a live Supervisor or an
// error. The caller must call Close() to release resources.
func Spawn(ctx context.Context, id, binary, workDir string, init protocol.PluginInit) (*Supervisor, error) {
	cmd := exec.CommandContext(ctx, binary) //nolint:gosec // path validated by manifest checksum
	cmd.Dir = workDir
	return spawnFromCmd(id, cmd, init)
}

// spawnFromCmd starts a pre-configured cmd and performs the PluginInit/PluginHello
// handshake. It is the implementation shared by Spawn and test helpers that
// inject extra environment variables.
func spawnFromCmd(id string, cmd *exec.Cmd, init protocol.PluginInit) (*Supervisor, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("oop: stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("oop: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("oop: start: %w", err)
	}

	s := newSupervisorFromConn(id, newConn(stdin, stdout))
	s.cmd = cmd
	go s.monitorProcess()

	if err := s.handshake(init); err != nil {
		_ = cmd.Process.Kill()
		return nil, err
	}
	return s, nil
}

// newSupervisorFromConn builds a Supervisor around an existing connection.
// Used by unit tests that inject in-process pipes instead of a real subprocess.
func newSupervisorFromConn(id string, c *conn) *Supervisor {
	return &Supervisor{
		id:   id,
		c:    c,
		done: make(chan struct{}),
	}
}

// IsFailed reports whether the subprocess is in the Failed state.
func (s *Supervisor) IsFailed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failed
}

// MarkFailed transitions the supervisor to Failed and closes the Done channel.
// Idempotent.
func (s *Supervisor) MarkFailed() {
	s.mu.Lock()
	s.failed = true
	s.mu.Unlock()
	s.once.Do(func() { close(s.done) })
}

// Close kills the subprocess (if any) and releases resources. Idempotent.
func (s *Supervisor) Close() {
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.MarkFailed()
}

// Done returns a channel closed when the subprocess exits or fails.
func (s *Supervisor) Done() <-chan struct{} { return s.done }

// DriveLifecycle sends a PluginLifecycle{Phase: phase} and waits for the
// corresponding PluginLifecycleDone (or PluginError). Any error marks the
// plugin Failed so the host detects the problem without crashing (INV-8).
func (s *Supervisor) DriveLifecycle(ctx context.Context, phase protocol.LifecyclePhase) error {
	if s.IsFailed() {
		return ErrPluginFailed
	}

	if err := s.c.send(protocol.PluginLifecycle{
		Type:  protocol.KindPluginLifecycle,
		Phase: phase,
	}); err != nil {
		s.MarkFailed()
		return fmt.Errorf("oop: send %s: %w", phase, err)
	}

	type reply struct {
		msg  any
		kind protocol.Kind
		err  error
	}
	ch := make(chan reply, 1)
	go func() {
		msg, kind, err := s.c.receive()
		ch <- reply{msg, kind, err}
	}()

	select {
	case <-ctx.Done():
		s.MarkFailed()
		return fmt.Errorf("oop: %s context cancelled: %w", phase, ctx.Err())
	case r := <-ch:
		if r.err != nil {
			s.MarkFailed()
			return fmt.Errorf("oop: receive after %s: %w", phase, r.err)
		}
		return s.checkLifecycleReply(r.msg, r.kind, phase)
	}
}

// checkLifecycleReply validates a reply to a DriveLifecycle call.
func (s *Supervisor) checkLifecycleReply(msg any, kind protocol.Kind, phase protocol.LifecyclePhase) error {
	switch m := msg.(type) {
	case protocol.PluginLifecycleDone:
		if m.Phase != phase {
			s.MarkFailed()
			return fmt.Errorf("oop: phase mismatch in Done (want %s, got %s)", phase, m.Phase)
		}
		return nil
	case protocol.PluginError:
		s.MarkFailed()
		return fmt.Errorf("oop: plugin error in %s: %s (code %d)", phase, m.Message, m.Code)
	default:
		s.MarkFailed()
		return fmt.Errorf("oop: unexpected message kind %q after %s", kind, phase)
	}
}

// handshake sends PluginInit and reads PluginHello within handshakeTimeout.
func (s *Supervisor) handshake(init protocol.PluginInit) error {
	init.Type = protocol.KindPluginInit
	if err := s.c.send(init); err != nil {
		return fmt.Errorf("%w: send PluginInit: %v", ErrHandshakeFailed, err)
	}

	type reply struct {
		msg  any
		kind protocol.Kind
		err  error
	}
	ch := make(chan reply, 1)
	go func() {
		msg, kind, err := s.c.receive()
		ch <- reply{msg, kind, err}
	}()

	timer := time.NewTimer(handshakeTimeout)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ErrHandshakeFailed
	case r := <-ch:
		if r.err != nil {
			return fmt.Errorf("%w: %v", ErrHandshakeFailed, r.err)
		}
		if r.kind != protocol.KindPluginHello {
			return fmt.Errorf("%w: unexpected kind %q (want PluginHello)", ErrHandshakeFailed, r.kind)
		}
		hello, ok := r.msg.(protocol.PluginHello)
		if !ok {
			return fmt.Errorf("%w: bad PluginHello payload", ErrHandshakeFailed)
		}
		slog.Info("oop plugin hello", "id", hello.ID, "version", hello.Version, "protocol", hello.ProtocolVersion)
		return nil
	}
}

// monitorProcess waits for the subprocess to exit and marks the supervisor
// Failed so the host detects the crash without polling (INV-8).
func (s *Supervisor) monitorProcess() {
	if s.cmd == nil {
		return
	}
	if err := s.cmd.Wait(); err != nil {
		slog.Warn("oop plugin process exited", "id", s.id, "err", err)
	}
	s.MarkFailed()
}
