package gametime

import (
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// TimePlugin registers all time resources and the time-update system.
// Resources registered: RealTime, VirtualTime, FixedTime, Time.
// System registered: updateTimeSystem in the First schedule.
type TimePlugin struct {
	// FixedPeriod sets the fixed-timestep period. Defaults to 1/60 s (60 Hz).
	FixedPeriod time.Duration
}

// Build implements appface.Plugin.
func (p TimePlugin) Build(app appface.Builder) {
	period := p.FixedPeriod
	if period <= 0 {
		period = time.Second / 60
	}
	now := time.Now()
	w := app.World()

	world.SetResource(w, newRealTime(now))
	world.SetResource(w, newVirtualTime())
	world.SetResource(w, NewFixedTime(period))
	world.SetResource(w, Time{startupTime: now})

	app.AddSystem(appface.First, scheduler.NewFuncSystem("gametime.UpdateTime", func(w *world.World) {
		rt, rtOK := world.Resource[RealTime](w)
		if !rtOK {
			return
		}
		rt.Advance(time.Now())

		vt, vtOK := world.Resource[VirtualTime](w)
		if vtOK {
			vt.Advance(rt.Delta())
		}

		ft, ftOK := world.Resource[FixedTime](w)
		if ftOK && vtOK {
			ft.Accumulate(vt.Delta())
		}

		if t, ok := world.Resource[Time](w); ok {
			if vtOK {
				t.delta = vt.Delta()
				t.elapsed = vt.Elapsed()
			}
			t.frameCount++
		}
	}))
}
