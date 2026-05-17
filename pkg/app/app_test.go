package app

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ── NewApp ────────────────────────────────────────────────────────────────────

func TestNewApp_NotNil(t *testing.T) {
	a := NewApp()
	if a == nil {
		t.Fatal("NewApp returned nil")
	}
	if a.World() == nil {
		t.Fatal("World is nil")
	}
}

// ── SetResource / InitResource ────────────────────────────────────────────────

type resCfg struct{ X int }

func TestApp_SetResource(t *testing.T) {
	a := NewApp()
	a.SetResource(resCfg{X: 42})
	v, ok := world.Resource[resCfg](a.World())
	if !ok {
		t.Fatal("resource not registered")
	}
	if v.X != 42 {
		t.Errorf("X = %d, want 42", v.X)
	}
}

func TestApp_InitResource_NoOverwrite(t *testing.T) {
	a := NewApp()
	a.InitResource(resCfg{X: 1})
	a.InitResource(resCfg{X: 2}) // must not overwrite
	v, _ := world.Resource[resCfg](a.World())
	if v.X != 1 {
		t.Errorf("X = %d, want 1 (InitResource must not overwrite)", v.X)
	}
}

// ── AddSystem / RunOnce ───────────────────────────────────────────────────────

func TestApp_AddSystem_RunOnce(t *testing.T) {
	ran := false
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("test.sys", func(_ *world.World) {
		ran = true
	}))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !ran {
		t.Error("system did not run")
	}
}

func TestApp_AddSystems(t *testing.T) {
	count := 0
	inc := func(_ *world.World) { count++ }
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddSystems(appface.Update,
		scheduler.NewFuncSystem("t1", inc),
		scheduler.NewFuncSystem("t2", inc),
	)
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

// ── Startup order ─────────────────────────────────────────────────────────────

func TestApp_StartupOrder(t *testing.T) {
	var order []string
	record := func(name string) scheduler.System {
		return scheduler.NewFuncSystem(name, func(_ *world.World) { order = append(order, name) })
	}
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddSystem(appface.PostStartup, record("PostStartup"))
	a.AddSystem(appface.Startup, record("Startup"))
	a.AddSystem(appface.PreStartup, record("PreStartup"))
	a.AddSystem(appface.Update, record("Update"))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	want := []string{"PreStartup", "Startup", "PostStartup", "Update"}
	if len(order) != len(want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	for i, s := range want {
		if order[i] != s {
			t.Errorf("order[%d] = %q, want %q", i, order[i], s)
		}
	}
}

// ── Exit / default runner ─────────────────────────────────────────────────────

func TestApp_Exit_StopsLoop(t *testing.T) {
	a := NewApp()
	frames := 0
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("counter", func(_ *world.World) {
		frames++
		if frames >= 3 {
			a.Exit()
		}
	}))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if frames != 3 {
		t.Errorf("frames = %d, want 3", frames)
	}
}

// ── Custom runner ─────────────────────────────────────────────────────────────

func TestApp_SetRunner(t *testing.T) {
	called := false
	a := NewApp()
	a.SetRunner(func(*App) error {
		called = true
		return nil
	})
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !called {
		t.Error("custom runner was not called")
	}
}

func TestApp_SetRunner_Error(t *testing.T) {
	want := errors.New("runner error")
	a := NewApp()
	a.SetRunner(func(*App) error { return want })
	if err := a.Run(); err != want {
		t.Errorf("err = %v, want %v", err, want)
	}
}

// ── AddPlugin dedup ───────────────────────────────────────────────────────────

type countPlugin struct{ count *int }

func (p countPlugin) Build(_ appface.Builder) { *p.count++ }

func TestApp_AddPlugin_Dedup(t *testing.T) {
	count := 0
	p := countPlugin{count: &count}
	a := NewApp()
	a.AddPlugin(p)
	a.AddPlugin(p) // second call must be a no-op
	if count != 1 {
		t.Errorf("Build called %d times, want 1", count)
	}
}

// ── AddPlugins ────────────────────────────────────────────────────────────────

type plugA struct{ built *int }

func (p plugA) Build(_ appface.Builder) { *p.built++ }

type plugB struct{ built *int }

func (p plugB) Build(_ appface.Builder) { *p.built++ }

func TestApp_AddPlugins(t *testing.T) {
	a, b := 0, 0
	pg := NewPluginGroup().Add(plugA{built: &a}).Add(plugB{built: &b})
	NewApp().AddPlugins(pg)
	if a != 1 || b != 1 {
		t.Errorf("built = {A:%d B:%d}, want {A:1 B:1}", a, b)
	}
}

// ── SubApp ────────────────────────────────────────────────────────────────────

func TestApp_SubApp_Lookup(t *testing.T) {
	a := NewApp()
	sub := NewSubApp()
	a.InsertSubApp("render", sub)

	if a.SubApp("render") != sub {
		t.Error("SubApp lookup failed")
	}
	if a.SubApp("missing") != nil {
		t.Error("missing SubApp should return nil")
	}
}

func TestApp_SubApp_RunsPerFrame(t *testing.T) {
	a := NewApp()
	sub := NewSubApp()
	subFrames := 0
	sub.AddSystem("SubUpdate", scheduler.NewFuncSystem("sub.sys", func(_ *world.World) {
		subFrames++
	}))
	a.InsertSubApp("sub", sub)

	frames := 0
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("main.exit", func(_ *world.World) {
		frames++
		if frames >= 2 {
			a.Exit()
		}
	}))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if subFrames != 2 {
		t.Errorf("subFrames = %d, want 2", subFrames)
	}
}

func TestApp_SubApp_Extract(t *testing.T) {
	type marker struct{ set bool }
	a := NewApp()
	world.SetResource(a.World(), marker{set: true})

	sub := NewSubApp()
	extracted := false
	sub.WithExtract(func(main, s *world.World) {
		m, ok := world.Resource[marker](main)
		if ok && m.set {
			extracted = true
		}
	})
	a.InsertSubApp("sub", sub)

	frames := 0
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("exit", func(_ *world.World) {
		frames++
		a.Exit()
	}))
	if err := a.Run(); err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if !extracted {
		t.Error("extract fn did not see main world resource")
	}
}

// ── PluginFn ──────────────────────────────────────────────────────────────────

func TestPluginFn_Build(t *testing.T) {
	called := false
	p := PluginFn(func(_ appface.Builder) { called = true })
	a := NewApp()
	a.AddPlugin(p)
	if !called {
		t.Error("PluginFn.Build was not called")
	}
}

// ── PluginGroupBuilder ────────────────────────────────────────────────────────

type firstPlugin struct{ order *[]string }

func (p firstPlugin) Build(_ appface.Builder) { *p.order = append(*p.order, "first") }

type secondPlugin struct{ order *[]string }

func (p secondPlugin) Build(_ appface.Builder) { *p.order = append(*p.order, "second") }

func TestPluginGroupBuilder_Order(t *testing.T) {
	var order []string
	pg := NewPluginGroup().Add(firstPlugin{&order}).Add(secondPlugin{&order})
	NewApp().AddPlugins(pg)
	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("order = %v, want [first second]", order)
	}
}

// ── DefaultPlugins / MinimalPlugins ──────────────────────────────────────────

func TestDefaultPlugins_Build(t *testing.T) {
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.AddPlugins(DefaultPlugins{})
	if err := a.Run(); err != nil {
		t.Fatalf("DefaultPlugins Run error: %v", err)
	}
}

func TestMinimalPlugins_Empty(t *testing.T) {
	if len(MinimalPlugins{}.Plugins()) != 0 {
		t.Error("MinimalPlugins should return empty slice")
	}
}

// ── Update public API ─────────────────────────────────────────────────────────

func TestApp_Update_DirectCall(t *testing.T) {
	ran := false
	a := NewApp()
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("t", func(_ *world.World) { ran = true }))
	if err := a.buildAllSchedules(); err != nil {
		t.Fatalf("buildAllSchedules: %v", err)
	}
	if err := a.Update(); err != nil {
		t.Fatalf("Update error: %v", err)
	}
	if !ran {
		t.Error("system did not run via Update")
	}
}

// ── SubApp.World ──────────────────────────────────────────────────────────────

func TestSubApp_World(t *testing.T) {
	sub := NewSubApp()
	if sub.World() == nil {
		t.Error("SubApp.World() returned nil")
	}
}

// ── SubApp.AddSystem ──────────────────────────────────────────────────────────

func TestSubApp_AddSystem_RunsOnUpdate(t *testing.T) {
	sub := NewSubApp()
	ran := false
	sub.AddSystem("SomeSchedule", scheduler.NewFuncSystem("sub.run", func(_ *world.World) {
		ran = true
	}))
	a := NewApp()
	a.SetRunMode(RunOnce)
	a.InsertSubApp("test", sub)
	if err := a.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ran {
		t.Error("SubApp system did not run")
	}
}

// ── Panic recovery in system propagates as error ──────────────────────────────

func TestApp_SystemPanicPropagatesAsError(t *testing.T) {
	a := NewApp()
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("panic.sys", func(_ *world.World) {
		panic("test panic")
	}))
	err := a.Run()
	if err == nil {
		t.Fatal("expected error from panicking system, got nil")
	}
}

func TestDefaultRunner_PropagatesSystemError(t *testing.T) {
	a := NewApp()
	frames := 0
	a.AddSystem(appface.Update, scheduler.NewFuncSystem("panic.sys", func(_ *world.World) {
		frames++
		if frames >= 2 {
			panic("deliberate panic in runner test")
		}
	}))
	err := a.Run()
	if err == nil {
		t.Fatal("expected error from panicking system in loop, got nil")
	}
}
