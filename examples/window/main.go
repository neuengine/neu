// examples/window validates the window subsystem end-to-end (T-6T06, C29 P6
// gate) against the headless backend: create → diff/apply → scripted event
// replay → close→exit decision (INV-2) → destroy, plus single-primary (INV-1).
// The hash over the backend call log is stable across ≥20 runs.
//
// Bootstrap: validates l2-window-system-go against l1-window-system.
package main

import (
	"fmt"

	"github.com/neuengine/neu/internal/ecs/entity"
	internalwindow "github.com/neuengine/neu/internal/window"
	"github.com/neuengine/neu/pkg/ecs"
	pkgwindow "github.com/neuengine/neu/pkg/window"
)

func ent(i uint32) ecs.Entity { return entity.FromID(entity.NewEntityID(i, 0)) }

func run() (uint64, error) {
	b := internalwindow.NewHeadlessWindowBackend()
	primary := ent(1)

	w := pkgwindow.DefaultWindow("Game")
	h, _ := b.CreateWindow(primary, pkgwindow.DescriptorFromWindow(w))
	if !h.IsValid() {
		return 0, fmt.Errorf("CreateWindow returned an invalid handle")
	}

	// Mutate → diff (INV-4) → apply. An empty diff would skip the backend call.
	w2 := w
	w2.Title = "Game (paused)"
	w2.Mode = pkgwindow.BorderlessFullscreen
	diff := internalwindow.DiffWindow(w, w2)
	if !diff.HasChanges() {
		return 0, fmt.Errorf("expected a non-empty diff after mutation")
	}
	_ = b.ApplyChanges(primary, diff)

	// INV-1: a second primary is rejected.
	var pr internalwindow.PrimaryWindowRes
	pr.SetPrimary(primary)
	if internalwindow.CheckSinglePrimary(2) == nil {
		return 0, fmt.Errorf("INV-1: two primaries must error")
	}

	// Scripted event replay → close-causes-exit decision (INV-2).
	b.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventResized, Window: primary, Width: 1024, Height: 768})
	b.ScriptEvents(pkgwindow.PlatformEvent{Kind: pkgwindow.EventCloseRequested, Window: primary})
	exit := false
	for {
		frame := b.PollEvents()
		if len(frame) == 0 {
			break
		}
		for _, ev := range frame {
			if pkgwindow.CausesAppExit(ev.Kind, pkgwindow.OnPrimaryClosed, pr.IsPrimary(ev.Window), 0) {
				exit = true
			}
		}
	}
	if !exit {
		return 0, fmt.Errorf("INV-2: primary close should cause AppExit")
	}

	_ = b.DestroyWindow(primary)
	if b.ActiveCount() != 0 {
		return 0, fmt.Errorf("destroy should leave zero active windows")
	}

	h2 := fnvOffset
	for _, c := range b.Calls() {
		h2 = hashStr(h2, c)
	}
	return h2, nil
}

const (
	fnvOffset uint64 = 14695981039346656037
	fnvPrime  uint64 = 1099511628211
)

func hashStr(h uint64, s string) uint64 {
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

func main() {
	h, err := run()
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return
	}
	fmt.Printf("PASS: window hash=%d\n", h)
}
