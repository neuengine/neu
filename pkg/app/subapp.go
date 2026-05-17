package app

import (
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

// SubApp is a secondary ECS context with its own World and schedules.
// It runs each frame after the main App update. An optional ExtractFn
// copies relevant state from the main World into the SubApp World before
// the sub-schedules execute (e.g. for a render SubApp).
type SubApp struct {
	w         *world.World
	order     []string
	schedules map[string]*scheduler.Schedule
	extract   ExtractFn
	built     bool
}

// NewSubApp creates a SubApp with an empty World and no schedules.
func NewSubApp() *SubApp {
	return &SubApp{
		w:         world.NewWorld(),
		schedules: make(map[string]*scheduler.Schedule),
	}
}

// World returns the SubApp's ECS World.
func (s *SubApp) World() *world.World { return s.w }

// WithExtract attaches an extract function that runs before this SubApp's
// schedules each frame.
func (s *SubApp) WithExtract(fn ExtractFn) *SubApp {
	s.extract = fn
	return s
}

// AddSystem registers a system in the named schedule, auto-creating it.
func (s *SubApp) AddSystem(name string, sys scheduler.System) *SubApp {
	sc, ok := s.schedules[name]
	if !ok {
		sc = scheduler.NewSchedule(name)
		s.schedules[name] = sc
		s.order = append(s.order, name)
	}
	_ = sc.AddSystem(sys)
	return s
}

// runSchedules builds (once) and runs all sub-schedules in insertion order.
func (s *SubApp) runSchedules(executor *scheduler.SequentialExecutor) error {
	if !s.built {
		for _, sc := range s.schedules {
			if err := sc.Build(); err != nil {
				return err
			}
		}
		s.built = true
	}
	for _, name := range s.order {
		if err := executor.Run(s.schedules[name], s.w); err != nil {
			return err
		}
	}
	return nil
}
