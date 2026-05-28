// Package audio provides the internal audio server, bus mixer, systems,
// and the default headless driver used for testing and CI (C-003, C29 P5 gate).
package audio

import (
	"sync"
	"sync/atomic"

	pkgaudio "github.com/neuengine/neu/pkg/audio"
)

// ── Headless driver ──────────────────────────────────────────────────────────

// HeadlessDriver is a zero-hardware AudioDriver implementation.
// It satisfies the interface without any OS audio handle.
// Used by default when no platform driver is registered.
type HeadlessDriver struct {
	mixRate  uint32
	channels uint32
	mu       sync.Mutex
}

func (d *HeadlessDriver) Init(mixRate, channels uint32) error {
	d.mixRate = mixRate
	d.channels = channels
	return nil
}
func (d *HeadlessDriver) Start()          {}
func (d *HeadlessDriver) MixRate() uint32 { return d.mixRate }
func (d *HeadlessDriver) Lock()           { d.mu.Lock() }
func (d *HeadlessDriver) Unlock()         { d.mu.Unlock() }
func (d *HeadlessDriver) Close() error    { return nil }

// ── Headless backend ─────────────────────────────────────────────────────────

// HeadlessBackend is a zero-hardware AudioBackend implementation.
// It tracks sink creation/drop/param updates for deterministic test assertions.
// All operations are goroutine-safe.
type HeadlessBackend struct {
	active       map[pkgaudio.SinkHandle]sinkState
	finished     []pkgaudio.SinkHandle
	seq          atomic.Uint64
	mu           sync.Mutex
	masterVolume float32
}

type sinkState struct {
	settings pkgaudio.SinkSettings
	params   pkgaudio.SinkParams
	dropped  bool
}

// NewHeadlessBackend returns a ready-to-use headless backend.
func NewHeadlessBackend() *HeadlessBackend {
	return &HeadlessBackend{
		active:       make(map[pkgaudio.SinkHandle]sinkState),
		masterVolume: 1,
	}
}

func (b *HeadlessBackend) CreateSink(s pkgaudio.SinkSettings, _ *pkgaudio.AudioSource) pkgaudio.SinkHandle {
	h := pkgaudio.SinkHandle(b.seq.Add(1))
	b.mu.Lock()
	b.active[h] = sinkState{settings: s}
	b.mu.Unlock()
	return h
}

func (b *HeadlessBackend) UpdateSink(h pkgaudio.SinkHandle, p pkgaudio.SinkParams) {
	b.mu.Lock()
	if st, ok := b.active[h]; ok {
		st.params = p
		b.active[h] = st
	}
	b.mu.Unlock()
}

func (b *HeadlessBackend) DropSink(h pkgaudio.SinkHandle) {
	b.mu.Lock()
	if st, ok := b.active[h]; ok {
		st.dropped = true
		b.active[h] = st
	}
	b.mu.Unlock()
}

func (b *HeadlessBackend) SetMasterVolume(v float32) {
	b.mu.Lock()
	b.masterVolume = v
	b.mu.Unlock()
}

func (b *HeadlessBackend) PollFinished() []pkgaudio.SinkHandle {
	b.mu.Lock()
	out := b.finished
	b.finished = nil
	b.mu.Unlock()
	return out
}

// FinishSink marks a sink as finished (simulates completion for testing).
func (b *HeadlessBackend) FinishSink(h pkgaudio.SinkHandle) {
	b.mu.Lock()
	b.finished = append(b.finished, h)
	b.mu.Unlock()
}

// ActiveCount returns the number of non-dropped sinks (for test assertions).
func (b *HeadlessBackend) ActiveCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, st := range b.active {
		if !st.dropped {
			count++
		}
	}
	return count
}

// MasterVolume returns the last value set via SetMasterVolume.
func (b *HeadlessBackend) MasterVolume() float32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.masterVolume
}
