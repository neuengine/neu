package net

import (
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/typereg"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// NetworkPlugin wires the networking stack into an App: transport resource,
// snapshot/input/rollback resources, and the receive/send systems. Opt-in
// (not in DefaultPlugins); add it to headless servers and clients alike.
//
//	app.AddPlugin(net.NetworkPlugin{Transport: udpTransport})
type NetworkPlugin struct {
	// Transport is the concrete NetworkTransport to inject. Required.
	Transport NetworkTransport
	// Reg is the component type registry used by SnapshotManager. If nil,
	// an empty registry is created (snapshots will capture zero components).
	Reg *typereg.TypeRegistry
	// SnapshotCapacity sets the snapshot ring size (0 → 16).
	SnapshotCapacity int
	// RngSeed seeds the DeterministicSchedule's per-tick RNG.
	RngSeed uint64
	// Timestep is the fixed simulation step (0 → 1/60 s).
	Timestep time.Duration
}

// Build implements appface.Plugin.
func (p NetworkPlugin) Build(app appface.Builder) {
	ts := p.Timestep
	if ts == 0 {
		ts = time.Second / 60
	}
	reg := p.Reg
	if reg == nil {
		reg = typereg.NewTypeRegistry()
	}

	snaps := NewSnapshotManager(reg, p.SnapshotCapacity)
	sched := NewDeterministicSchedule(p.RngSeed, ts)
	rc := NewRollbackCoordinator(snaps, sched)
	buf := NewInputBuffer()

	w := app.World()
	world.SetResource(w, TransportResource{T: p.Transport})
	world.SetResource(w, snaps)
	world.SetResource(w, sched)
	world.SetResource(w, rc)
	world.SetResource(w, buf)
	world.SetResource(w, InboundQueue{})
	world.SetResource(w, OutboundQueue{})

	app.
		AddSystem(appface.PreUpdate, scheduler.NewFuncSystem("net.NetworkReceive", networkReceive)).
		AddSystem(appface.PostUpdate, scheduler.NewFuncSystem("net.NetworkSend", networkSend))
}
