package predict

import (
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	netcore "github.com/neuengine/neu/internal/net"
	"github.com/neuengine/neu/pkg/app/appface"
)

// PredictionPlugin wires the client-prediction stack into an App. Add it after
// NetworkPlugin (which provides *RollbackCoordinator, *SnapshotManager,
// *InputBuffer, and OutboundQueue).
//
//	app.AddPlugin(predict.PredictionPlugin{
//	    LocalPlayer: 1,
//	    ServerConn:  0,
//	})
type PredictionPlugin struct {
	// LocalPlayer is the PlayerID used for input recording and packet tagging.
	LocalPlayer netcore.PlayerID
	// ServerConn is the ConnectionID of the authoritative server; input packets
	// are addressed to it on ChannelEvents.
	ServerConn netcore.ConnectionID
	// HistoryCapacity is the depth of the prediction ring (0 → DefaultHistoryCapacity).
	HistoryCapacity int
	// BlendTimestep is the per-frame step for correction smoothing (0 → 1/60 s).
	BlendTimestep time.Duration
}

// Build implements appface.Plugin.
func (p PredictionPlugin) Build(app appface.Builder) {
	w := app.World()

	// Pre-register component types so ECS ID assignment is deterministic.
	world.RegisterComponent[NetworkAuthority](w)
	world.RegisterComponent[CorrectionState](w)

	// ServerState resource consumed by ReconciliationSystem each PreUpdate.
	world.SetResource(w, ServerState{})

	hist := NewPredictionHistory(p.HistoryCapacity)
	app.
		AddSystem(appface.FixedUpdate,
			scheduler.NewFuncSystem("predict.PredictionSystem",
				NewPredictionSystem(p.LocalPlayer, p.ServerConn, hist).Run)).
		AddSystem(appface.PreUpdate,
			scheduler.NewFuncSystem("predict.ReconciliationSystem",
				NewReconciliationSystem(hist).Run)).
		AddSystem(appface.Last,
			scheduler.NewFuncSystem("predict.CorrectionSmoothingSystem",
				NewCorrectionSmoothingSystem(p.BlendTimestep).Run))
}
