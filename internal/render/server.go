// Package render (internal) holds the engine-private render core: the
// command-queue server, resource tracker, and (later tasks) the render graph,
// SubApp extract, and feature pipeline. Callers use the public pkg/render
// surface; nothing outside the engine imports this package.
//
// Bootstrap 0.1.0 — implements l2-render-core-go.md §Server / §Type
// Definitions. l1-render-core / l2-render-core-go are Draft (C29 P4 gate open);
// this is a [Bootstrap] artifact and may be revised when the specs finalize.
package render

import (
	"errors"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	gpu "github.com/neuengine/neu/pkg/render"
)

// ErrRenderClosed is returned by [Server.Submit] / [Server.Initialize] after
// [Server.Close]. Work is never silently dropped (mirrors task.ErrPoolClosed).
var ErrRenderClosed = errors.New("render: server is closed")

// command is a unit of deferred GPU work run on the server goroutine.
type command func()

// Server serialises all frontend→backend calls (l1-render-core §4.5). RIDs are
// allocated synchronously; mutation is queued. Calls made from the bound
// server goroutine execute directly (no queue hop); calls from any other
// goroutine are pushed onto a mutex-guarded queue and drained by [Server.Drain].
type Server struct {
	backend gpu.RenderBackend

	mu     sync.Mutex
	queue  []command
	closed bool

	nextIndex atomic.Uint64 // dense RID index source
	srvGID    atomic.Int64  // 0 = unbound; set by Bind
}

// NewServer returns a Server fronting backend. Call [Server.Bind] from the
// render goroutine before the first [Server.Drain].
func NewServer(backend gpu.RenderBackend) *Server {
	return &Server{backend: backend}
}

// Bind records the calling goroutine as the server goroutine. Submissions from
// this goroutine bypass the queue and run inline (l1-render-core §4.5).
func (s *Server) Bind() { s.srvGID.Store(currentGID()) }

// Allocate hands back an RID immediately (l1-render-core §4.5 two-phase
// create, step 1). No backend object exists yet — pair with [Server.Initialize].
// Generation is 1 in 0.1.0 (reuse/free-list is a later task; no scope creep).
func (s *Server) Allocate(kind gpu.ResourceKind) gpu.RID {
	idx := uint32(s.nextIndex.Add(1) - 1)
	return gpu.MakeRID(kind, idx, 1)
}

// Initialize queues backend creation/upload for rid (two-phase create, step 2).
// Returns ErrRenderClosed if the server is closed. The caller may use rid in
// further queued commands immediately — no stall on completion.
func (s *Server) Initialize(rid gpu.RID, init func(b gpu.RenderBackend)) error {
	return s.Submit(func() { init(s.backend) })
}

// Submit runs cmd on the server goroutine. If the caller IS the server
// goroutine, cmd runs inline (FIFO preserved: the queue is drained before any
// inline call by [Server.Drain]). Otherwise cmd is enqueued. Returns
// ErrRenderClosed if the server is closed (work is never dropped silently).
func (s *Server) Submit(cmd command) error {
	if id := s.srvGID.Load(); id != 0 && currentGID() == id {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return ErrRenderClosed
		}
		// Preserve global FIFO: drain anything queued by other goroutines
		// before running this inline command.
		pending := s.queue
		s.queue = nil
		s.mu.Unlock()
		for _, c := range pending {
			c()
		}
		cmd()
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return ErrRenderClosed
	}
	s.queue = append(s.queue, cmd)
	return nil
}

// Drain runs every queued command in submission order. Must be called from the
// bound (Bind) goroutine — it is the only consumer of the queue.
func (s *Server) Drain() {
	for {
		s.mu.Lock()
		if len(s.queue) == 0 {
			s.mu.Unlock()
			return
		}
		batch := s.queue
		s.queue = nil
		s.mu.Unlock()
		for _, c := range batch {
			c()
		}
	}
}

// Close marks the server closed. Subsequent Submit/Initialize calls return
// ErrRenderClosed. Already-queued commands are not run after Close.
func (s *Server) Close() {
	s.mu.Lock()
	s.closed = true
	s.queue = nil
	s.mu.Unlock()
}

// currentGID returns the calling goroutine's ID by parsing a minimal stack
// trace. Used only by the Bind/Submit fast-path check — never on a hot loop.
// Mirrors the pkg/task pattern (C30 reference utilisation).
func currentGID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	s := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	if i := strings.IndexByte(s, ' '); i > 0 {
		id, _ := strconv.ParseInt(s[:i], 10, 64)
		return id
	}
	return -1
}
