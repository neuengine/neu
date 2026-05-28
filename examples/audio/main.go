// examples/audio demonstrates the audio system's bus-graph routing,
// spatial listener attenuation, and DESPAWN-mode determinism using the
// headless stub backend (zero hardware, C-003, C29 P5 gate).
//
// Bootstrap: validates l2-audio-system-go against l1-audio-system.
package main

import (
	"fmt"

	internalaudio "github.com/neuengine/neu/internal/audio"
	pkgaudio "github.com/neuengine/neu/pkg/audio"
)

// run exercises the audio system and returns a deterministic hash of the
// backend's final state for stability testing.
func run() (uint64, error) {
	// Set up a headless driver + backend + server (C-003, C29 gate).
	driver := &internalaudio.HeadlessDriver{}
	if err := driver.Init(44100, 2); err != nil {
		return 0, fmt.Errorf("driver init: %w", err)
	}
	driver.Start()

	backend := internalaudio.NewHeadlessBackend()
	srv := internalaudio.NewAudioServer(driver, backend)

	// Configure bus graph: Master → Music, SFX, Ambient (INV-bus-DAG).
	layout := pkgaudio.AudioBusLayout{
		Buses: []pkgaudio.AudioBus{
			{Name: "Master", Volume: 1, Output: ""},
			{Name: "Music", Volume: 0.8, Output: "Master"},
			{Name: "SFX", Volume: 1, Output: "Master"},
			{Name: "Ambient", Volume: 0.6, Output: "Master"},
		},
	}
	if err := srv.SetLayout(layout); err != nil {
		return 0, fmt.Errorf("bus layout: %w", err)
	}

	// Create sinks on various buses.
	src := &pkgaudio.AudioSource{SampleRate: 44100, Channels: 2, Samples: []float32{0}}

	h1 := backend.CreateSink(pkgaudio.SinkSettings{Mode: pkgaudio.PlaybackOnce, Volume: 0.8, Bus: "Music"}, src)
	h2 := backend.CreateSink(pkgaudio.SinkSettings{Mode: pkgaudio.PlaybackLoop, Volume: 1, Bus: "SFX"}, src)
	h3 := backend.CreateSink(pkgaudio.SinkSettings{Mode: pkgaudio.PlaybackDespawn, Volume: 0.6, Bus: "Ambient"}, src)

	if h1.IsNil() || h2.IsNil() || h3.IsNil() {
		return 0, fmt.Errorf("sink creation failed")
	}

	// Simulate spatial attenuation check.
	atten := internalaudio.Attenuation(0, 10, 1, 100) // inverse distance at dist=10
	if atten <= 0 || atten > 1 {
		return 0, fmt.Errorf("attenuation out of range: %v", atten)
	}

	// Simulate DESPAWN-mode: backend finishes h3.
	backend.FinishSink(h3)
	finished := backend.PollFinished()
	if len(finished) != 1 || finished[0] != h3 {
		return 0, fmt.Errorf("DESPAWN: expected h3 finished, got %v", finished)
	}

	// Drop non-despawn sinks.
	backend.DropSink(h1)
	backend.DropSink(h2)
	backend.DropSink(h3)

	// Compute a deterministic hash from the final state.
	hash := fnv1a64(uint64(backend.ActiveCount()), uint64(layout.Buses[0].Volume*1000))
	return hash, nil
}

// fnv1a64 is a simple hash combiner for determinism testing.
func fnv1a64(vals ...uint64) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := uint64(offset64)
	for _, v := range vals {
		h ^= v
		h *= prime64
	}
	return h
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: audio example hash=%d\n", h)
}
