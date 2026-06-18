package lockstep

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/app/appface"
)

// LockstepPlugin wires the deterministic lockstep stack into an App. Add it
// after NetworkPlugin (which provides *RollbackCoordinator, *SnapshotManager,
// *InputBuffer, OutboundQueue, and InboundQueue).
//
//	app.AddPlugin(lockstep.LockstepPlugin{
//	    Config:      lockstep.LockstepConfig{Peers: []netcore.PlayerID{1, 2}},
//	    LocalPlayer: 1,
//	    IsServer:    true,
//	})
type LockstepPlugin struct {
	// Config is passed to NewLockstepScheduler; zero fields use defaults.
	Config LockstepConfig
	// LocalPlayer is the PlayerID whose input LocalInputSystem captures.
	LocalPlayer netcore.PlayerID
	// IsServer controls which LatejoinSystem path runs:
	// true = send snapshot to joining peers; false = receive snapshot from host.
	IsServer bool
	// Speculative configures opt-in speculative execution (off by default).
	Speculative SpeculativeConfig
}

// Build implements appface.Plugin.
func (p LockstepPlugin) Build(app appface.Builder) {
	ls := NewLockstepScheduler(p.Config)

	localSys := NewLocalInputSystem(p.LocalPlayer, p.Config.InputDelay)
	remoteSys := &RemoteInputSystem{}
	desyncSys := NewDesyncReceiveSystem(ls)
	specSys := NewSpeculativeExecutor(ls, p.Speculative)
	latejoinSys := NewLatejoinSystem(ls, p.IsServer)

	app.
		AddSystem(appface.PreUpdate,
			scheduler.NewFuncSystem("lockstep.RemoteInputSystem", remoteSys.Run)).
		AddSystem(appface.PreUpdate,
			scheduler.NewFuncSystem("lockstep.LatejoinSystem", latejoinSys.Run)).
		AddSystem(appface.FixedPreUpdate,
			scheduler.NewFuncSystem("lockstep.SpeculativeExecutor", specSys.Run)).
		AddSystem(appface.FixedUpdate,
			scheduler.NewFuncSystem("lockstep.LocalInputSystem", localSys.Run)).
		AddSystem(appface.FixedUpdate,
			scheduler.NewFuncSystem("lockstep.LockstepScheduler", ls.Run)).
		AddSystem(appface.PostUpdate,
			scheduler.NewFuncSystem("lockstep.DesyncReceiveSystem", desyncSys.Run))
}
