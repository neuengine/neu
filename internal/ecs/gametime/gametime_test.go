package gametime

import (
	"testing"
	"time"

	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
)

// ── mockBuilder allows testing TimePlugin.Build without a full App. ─────────

type mockBuilder struct {
	w       *world.World
	systems map[string][]scheduler.System
}

func newMockBuilder() *mockBuilder {
	return &mockBuilder{
		w:       world.NewWorld(),
		systems: make(map[string][]scheduler.System),
	}
}

func (m *mockBuilder) World() *world.World { return m.w }
func (m *mockBuilder) AddSystem(sched string, sys scheduler.System) appface.Builder {
	m.systems[sched] = append(m.systems[sched], sys)
	return m
}
func (m *mockBuilder) AddSystems(sched string, syss ...scheduler.System) appface.Builder {
	m.systems[sched] = append(m.systems[sched], syss...)
	return m
}
func (m *mockBuilder) SetResource(_ any) appface.Builder                { return m }
func (m *mockBuilder) InitResource(_ any) appface.Builder               { return m }
func (m *mockBuilder) AddPlugin(_ appface.Plugin) appface.Builder       { return m }
func (m *mockBuilder) AddPlugins(_ appface.PluginGroup) appface.Builder { return m }

// ── RealTime ─────────────────────────────────────────────────────────────────

func TestRealTimeAdvance(t *testing.T) {
	start := time.Now()
	rt := newRealTime(start)

	later := start.Add(16 * time.Millisecond)
	rt.Advance(later)

	if rt.Delta() != 16*time.Millisecond {
		t.Fatalf("delta want 16ms got %v", rt.Delta())
	}
	if rt.Elapsed() != 16*time.Millisecond {
		t.Fatalf("elapsed want 16ms got %v", rt.Elapsed())
	}
	if rt.DeltaSeconds() < 0.015 || rt.DeltaSeconds() > 0.017 {
		t.Fatalf("DeltaSeconds out of range: %v", rt.DeltaSeconds())
	}
	if rt.ElapsedSeconds() < 0.015 || rt.ElapsedSeconds() > 0.017 {
		t.Fatalf("ElapsedSeconds out of range: %v", rt.ElapsedSeconds())
	}
	if rt.StartupTime() != start {
		t.Fatal("StartupTime mismatch")
	}
}

func TestRealTimeMultiAdvance(t *testing.T) {
	start := time.Now()
	rt := newRealTime(start)
	rt.Advance(start.Add(10 * time.Millisecond))
	rt.Advance(start.Add(20 * time.Millisecond))
	// second advance: delta = 10ms, total elapsed = 20ms
	if rt.Delta() != 10*time.Millisecond {
		t.Fatalf("second delta want 10ms got %v", rt.Delta())
	}
	if rt.Elapsed() != 20*time.Millisecond {
		t.Fatalf("elapsed want 20ms got %v", rt.Elapsed())
	}
}

// ── VirtualTime ───────────────────────────────────────────────────────────────

func TestVirtualScaling(t *testing.T) {
	vt := newVirtualTime()

	vt.SetRelativeSpeed(0)
	vt.Advance(100 * time.Millisecond)
	if vt.Delta() != 0 {
		t.Fatalf("speed=0 delta want 0 got %v", vt.Delta())
	}
	if vt.Elapsed() != 0 {
		t.Fatalf("speed=0 elapsed want 0 got %v", vt.Elapsed())
	}
	if vt.DeltaSeconds() != 0 {
		t.Fatalf("DeltaSeconds want 0 got %v", vt.DeltaSeconds())
	}

	vt.SetRelativeSpeed(2)
	if vt.RelativeSpeed() != 2 {
		t.Fatalf("RelativeSpeed want 2 got %v", vt.RelativeSpeed())
	}
	vt.Advance(100 * time.Millisecond)
	if vt.Delta() != 200*time.Millisecond {
		t.Fatalf("speed=2 delta want 200ms got %v", vt.Delta())
	}
	if vt.Elapsed() != 200*time.Millisecond {
		t.Fatalf("elapsed want 200ms got %v", vt.Elapsed())
	}
}

func TestVirtualTimePause(t *testing.T) {
	vt := newVirtualTime()
	if vt.IsPaused() {
		t.Fatal("should not start paused")
	}
	vt.Pause()
	if !vt.IsPaused() {
		t.Fatal("should be paused")
	}
	vt.Advance(50 * time.Millisecond)
	if vt.Delta() != 0 || vt.Elapsed() != 0 {
		t.Fatal("paused virtual time must not advance")
	}

	vt.Unpause()
	if vt.IsPaused() {
		t.Fatal("should not be paused after Unpause")
	}
	vt.Advance(20 * time.Millisecond)
	if vt.Delta() != 20*time.Millisecond {
		t.Fatalf("unpaused delta want 20ms got %v", vt.Delta())
	}
}

func TestVirtualTimeNegativeSpeedClamped(t *testing.T) {
	vt := newVirtualTime()
	vt.SetRelativeSpeed(-5)
	if vt.RelativeSpeed() != 0 {
		t.Fatalf("negative speed not clamped: got %v", vt.RelativeSpeed())
	}
}

// ── FixedTime ─────────────────────────────────────────────────────────────────

func TestFixedStepDeterminism(t *testing.T) {
	period := 16 * time.Millisecond
	ft := NewFixedTime(period)

	ft.Accumulate(50 * time.Millisecond)
	steps := 0
	for ft.Expend() {
		steps++
	}
	if steps != 3 {
		t.Fatalf("want 3 steps got %d", steps)
	}
	if ft.Elapsed() != 3*period {
		t.Fatalf("elapsed want %v got %v", 3*period, ft.Elapsed())
	}
	if ft.Overstep() < time.Millisecond || ft.Overstep() > 3*time.Millisecond {
		t.Fatalf("overstep want ~2ms got %v", ft.Overstep())
	}
	if ft.OverstepFraction() < 0.1 || ft.OverstepFraction() > 0.2 {
		t.Fatalf("OverstepFraction out of range: %v", ft.OverstepFraction())
	}
	if ft.Period() != period {
		t.Fatalf("Period want %v got %v", period, ft.Period())
	}
	if ft.PeriodSeconds() < 0.015 || ft.PeriodSeconds() > 0.017 {
		t.Fatalf("PeriodSeconds out of range: %v", ft.PeriodSeconds())
	}
}

func TestFixedStepDeathSpiral(t *testing.T) {
	ft := NewFixedTime(16 * time.Millisecond)
	ft.Accumulate(5 * time.Second)
	if ft.Accumulated() > DefaultMaxAccumulation {
		t.Fatalf("accumulated %v exceeds cap %v", ft.Accumulated(), DefaultMaxAccumulation)
	}
}

func TestFixedStepZeroExpend(t *testing.T) {
	ft := NewFixedTime(16 * time.Millisecond)
	ft.Accumulate(10 * time.Millisecond)
	if ft.Expend() {
		t.Fatal("Expend must return false when accumulator < period")
	}
}

func TestFixedTimeSetPeriod(t *testing.T) {
	ft := NewFixedTime(16 * time.Millisecond)
	ft.SetPeriod(8 * time.Millisecond)
	if ft.Period() != 8*time.Millisecond {
		t.Fatalf("SetPeriod: want 8ms got %v", ft.Period())
	}
}

func TestFixedTimeSetPeriodPanic(t *testing.T) {
	ft := NewFixedTime(16 * time.Millisecond)
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on zero period in SetPeriod")
		}
	}()
	ft.SetPeriod(0)
}

func TestFixedTimeNewPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on zero period in NewFixedTime")
		}
	}()
	NewFixedTime(0)
}

// ── Timer ─────────────────────────────────────────────────────────────────────

func TestTimerOnce(t *testing.T) {
	tmr := NewTimer(time.Second, TimerOnce)

	tmr.Tick(500 * time.Millisecond)
	if tmr.Finished() {
		t.Fatal("should not finish at 500ms")
	}
	if tmr.JustFinished() {
		t.Fatal("JustFinished must be false before completion")
	}
	if tmr.Fraction() < 0.49 || tmr.Fraction() > 0.51 {
		t.Fatalf("fraction want ~0.5 got %v", tmr.Fraction())
	}
	if tmr.FractionRemaining() < 0.49 || tmr.FractionRemaining() > 0.51 {
		t.Fatalf("FractionRemaining want ~0.5 got %v", tmr.FractionRemaining())
	}
	if tmr.Remaining() < 499*time.Millisecond || tmr.Remaining() > 501*time.Millisecond {
		t.Fatalf("Remaining want ~500ms got %v", tmr.Remaining())
	}

	// cross finish boundary
	tmr.Tick(600 * time.Millisecond)
	if !tmr.Finished() {
		t.Fatal("must be finished after 1.1s total")
	}
	if !tmr.JustFinished() {
		t.Fatal("JustFinished must be true the frame it completes")
	}
	if tmr.TimesFinished() != 1 {
		t.Fatalf("TimesFinished want 1 got %d", tmr.TimesFinished())
	}
	if tmr.Remaining() != 0 {
		t.Fatalf("Remaining want 0 got %v", tmr.Remaining())
	}
	if tmr.Fraction() != 1 {
		t.Fatalf("Fraction want 1 got %v", tmr.Fraction())
	}
	if tmr.FractionRemaining() != 0 {
		t.Fatalf("FractionRemaining want 0 got %v", tmr.FractionRemaining())
	}

	// next tick: JustFinished must be false (exactly one-frame rule)
	tmr.Tick(16 * time.Millisecond)
	if tmr.JustFinished() {
		t.Fatal("JustFinished must be false after the completion frame")
	}
	if !tmr.Finished() {
		t.Fatal("Finished must remain true")
	}
}

func TestTimerRepeating(t *testing.T) {
	tmr := NewTimer(100*time.Millisecond, TimerRepeating)
	tmr.Tick(350 * time.Millisecond)
	if tmr.TimesFinished() != 3 {
		t.Fatalf("TimesFinished want 3 got %d", tmr.TimesFinished())
	}
	if !tmr.JustFinished() {
		t.Fatal("JustFinished must be true")
	}
	// elapsed remainder = 50ms, fraction ≈ 0.5
	if tmr.Fraction() < 0.49 || tmr.Fraction() > 0.51 {
		t.Fatalf("fraction want ~0.5 got %v", tmr.Fraction())
	}
}

func TestTimerReset(t *testing.T) {
	tmr := NewTimer(time.Second, TimerOnce)
	tmr.Tick(2 * time.Second)
	if !tmr.Finished() {
		t.Fatal("must be finished before reset")
	}
	tmr.Reset()
	if tmr.Finished() || tmr.JustFinished() || tmr.Fraction() != 0 {
		t.Fatal("all states should reset to zero")
	}
}

func TestTimerPause(t *testing.T) {
	tmr := NewTimer(time.Second, TimerOnce)
	if tmr.IsPaused() {
		t.Fatal("should not start paused")
	}
	tmr.Pause()
	if !tmr.IsPaused() {
		t.Fatal("should be paused")
	}
	tmr.Tick(2 * time.Second)
	if tmr.Finished() {
		t.Fatal("paused timer must not finish")
	}
	tmr.Unpause()
	tmr.Tick(time.Second)
	if !tmr.Finished() {
		t.Fatal("unpaused timer must finish after duration")
	}
}

// ── Stopwatch ─────────────────────────────────────────────────────────────────

func TestStopwatch(t *testing.T) {
	sw := NewStopwatch()
	sw.Tick(100 * time.Millisecond)
	sw.Tick(200 * time.Millisecond)
	if sw.Elapsed() != 300*time.Millisecond {
		t.Fatalf("elapsed want 300ms got %v", sw.Elapsed())
	}
	if sw.ElapsedSeconds() < 0.299 || sw.ElapsedSeconds() > 0.301 {
		t.Fatalf("ElapsedSeconds out of range: %v", sw.ElapsedSeconds())
	}

	sw.Pause()
	if !sw.IsPaused() {
		t.Fatal("should be paused")
	}
	sw.Tick(500 * time.Millisecond)
	if sw.Elapsed() != 300*time.Millisecond {
		t.Fatal("paused stopwatch must not advance")
	}

	sw.Unpause()
	sw.Tick(100 * time.Millisecond)
	if sw.Elapsed() != 400*time.Millisecond {
		t.Fatalf("after unpause: want 400ms got %v", sw.Elapsed())
	}

	sw.Reset()
	if sw.Elapsed() != 0 {
		t.Fatal("elapsed should be 0 after reset")
	}
}

// ── TimePlugin ────────────────────────────────────────────────────────────────

func TestTimePluginBuild(t *testing.T) {
	mb := newMockBuilder()
	p := TimePlugin{FixedPeriod: 16 * time.Millisecond}
	p.Build(mb)

	w := mb.w
	if _, ok := world.Resource[RealTime](w); !ok {
		t.Fatal("RealTime resource not registered")
	}
	if _, ok := world.Resource[VirtualTime](w); !ok {
		t.Fatal("VirtualTime resource not registered")
	}
	if _, ok := world.Resource[FixedTime](w); !ok {
		t.Fatal("FixedTime resource not registered")
	}
	if _, ok := world.Resource[Time](w); !ok {
		t.Fatal("Time resource not registered")
	}
	if len(mb.systems[appface.First]) != 1 {
		t.Fatalf("expected 1 First system got %d", len(mb.systems[appface.First]))
	}
}

func TestTimePluginDefaultPeriod(t *testing.T) {
	mb := newMockBuilder()
	TimePlugin{}.Build(mb) // zero FixedPeriod → defaults to 1/60s

	ft, ok := world.Resource[FixedTime](mb.w)
	if !ok {
		t.Fatal("FixedTime resource not registered")
	}
	expected := time.Second / 60
	if ft.Period() != expected {
		t.Fatalf("default period want %v got %v", expected, ft.Period())
	}
}

func TestTimePluginUpdateSystem(t *testing.T) {
	mb := newMockBuilder()
	TimePlugin{FixedPeriod: 16 * time.Millisecond}.Build(mb)

	sys := mb.systems[appface.First][0]
	sys.Run(mb.w)
	sys.Run(mb.w)

	tr, ok := world.Resource[Time](mb.w)
	if !ok {
		t.Fatal("Time resource missing")
	}
	if tr.FrameCount() < 2 {
		t.Fatalf("frameCount want ≥2 got %d", tr.FrameCount())
	}
	// Exercise the Time convenience methods (all five).
	_ = tr.Delta()
	_ = tr.DeltaSeconds()
	_ = tr.Elapsed()
	_ = tr.ElapsedSeconds()
	if tr.StartupTime().IsZero() {
		t.Fatal("StartupTime must not be zero")
	}
}

func TestTimerZeroDuration(t *testing.T) {
	// Zero-duration timer: Fraction must return 1 immediately.
	tmr := NewTimer(0, TimerOnce)
	if tmr.Fraction() != 1 {
		t.Fatalf("zero-duration Fraction want 1 got %v", tmr.Fraction())
	}
	// Tick finishes it immediately.
	tmr.Tick(0)
	if !tmr.Finished() {
		t.Fatal("zero-duration timer must finish on first Tick")
	}
}

func TestFixedTimeOverstepFractionZero(t *testing.T) {
	ft := NewFixedTime(16 * time.Millisecond)
	// Accumulate exactly one period and expend it — overstep becomes 0.
	ft.Accumulate(16 * time.Millisecond)
	ft.Expend()
	// Now Expend returns false and sets overstep = accumulated (0).
	ft.Expend()
	if ft.OverstepFraction() != 0 {
		t.Fatalf("OverstepFraction want 0 got %v", ft.OverstepFraction())
	}
}
