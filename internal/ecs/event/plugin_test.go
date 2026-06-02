package event_test

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/event"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

type ping struct{ n int }

// captureBuilder is a minimal appface.Builder recording the systems a plugin
// registers, so EventsPlugin.Build can be tested without importing pkg/app.
type captureBuilder struct {
	w       *world.World
	systems map[string][]scheduler.System
}

func newCaptureBuilder() *captureBuilder {
	return &captureBuilder{w: world.NewWorld(), systems: map[string][]scheduler.System{}}
}

func (b *captureBuilder) World() *world.World { return b.w }
func (b *captureBuilder) AddSystem(schedule string, s scheduler.System) appface.Builder {
	b.systems[schedule] = append(b.systems[schedule], s)
	return b
}
func (b *captureBuilder) AddSystems(schedule string, ss ...scheduler.System) appface.Builder {
	for _, s := range ss {
		b.AddSystem(schedule, s)
	}
	return b
}
func (b *captureBuilder) SetResource(any) appface.Builder            { return b }
func (b *captureBuilder) InitResource(any) appface.Builder           { return b }
func (b *captureBuilder) AddPlugin(appface.Plugin) appface.Builder   { return b }
func (b *captureBuilder) AddPlugins(appface.PluginGroup) appface.Builder { return b }

var _ appface.Builder = (*captureBuilder)(nil)

func TestEventsPluginSwapsEachFrame(t *testing.T) {
	t.Parallel()
	b := newCaptureBuilder()
	event.EventsPlugin{}.Build(b)

	sys := b.systems[appface.First]
	if len(sys) != 1 || sys[0].Name() != "ecs.SwapEvents" {
		t.Fatalf("First-schedule systems = %v, want [ecs.SwapEvents]", sys)
	}
	swap := sys[0]

	bus := event.RegisterEvent[ping](b.w)
	bus.Send(ping{n: 1})
	if bus.Len() != 1 {
		t.Fatalf("after Send, Len = %d, want 1", bus.Len())
	}

	// One swap: the event moves to the "previous" buffer but is still readable
	// (the two-frame retention contract).
	swap.Run(b.w)
	if bus.Len() != 1 {
		t.Errorf("after 1 swap, Len = %d, want 1 (still retained)", bus.Len())
	}

	// Second swap with no new sends: the event is cleared.
	swap.Run(b.w)
	if bus.Len() != 0 {
		t.Errorf("after 2 swaps, Len = %d, want 0 (cleared)", bus.Len())
	}

	// Idle swaps stay at zero and never panic.
	swap.Run(b.w)
	if bus.Len() != 0 {
		t.Errorf("idle swap, Len = %d, want 0", bus.Len())
	}
}

func TestEventsPluginNoRegistryNoOp(t *testing.T) {
	t.Parallel()
	// With no event registered, the swap system is a no-op (empty registry) and
	// must not panic.
	b := newCaptureBuilder()
	event.EventsPlugin{}.Build(b)
	b.systems[appface.First][0].Run(b.w)
}
