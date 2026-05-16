package scheduler_test

import (
	"errors"
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/query"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
)

// fixture component for access declarations.
type schedPos struct{ X int }
type schedVel struct{ DX int }

func registerComponentID[T any](t *testing.T, w *world.World) component.ID {
	t.Helper()
	return component.RegisterType[T](w.Components())
}

func newSys(name string) *scheduler.FuncSystem {
	return scheduler.NewFuncSystem(name, nil)
}

func newSysWithReadAccess(t *testing.T, w *world.World, name string, ids ...component.ID) *scheduler.FuncSystem {
	t.Helper()
	var a query.Access
	for _, id := range ids {
		a.AddRead(id)
	}
	return scheduler.NewFuncSystem(name, nil).WithAccess(a)
}

func newSysWithWriteAccess(t *testing.T, w *world.World, name string, ids ...component.ID) *scheduler.FuncSystem {
	t.Helper()
	var a query.Access
	for _, id := range ids {
		a.AddWrite(id)
	}
	return scheduler.NewFuncSystem(name, nil).WithAccess(a)
}

func TestNewScheduleEmpty(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	if s.Name() != "Update" {
		t.Fatalf("Name = %q, want Update", s.Name())
	}
	if s.SystemCount() != 0 {
		t.Fatalf("SystemCount = %d, want 0", s.SystemCount())
	}
	if err := s.Build(); err != nil {
		t.Fatalf("empty Build err = %v", err)
	}
	if got := s.Order(); len(got) != 0 {
		t.Fatalf("empty Order = %v, want []", got)
	}
	if got := s.SystemsInOrder(); len(got) != 0 {
		t.Fatalf("empty SystemsInOrder = %v, want []", got)
	}
}

func TestAddSystemAssignsIDs(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	a := s.AddSystem(newSys("A"))
	b := s.AddSystem(newSys("B"))
	if a.Err() != nil || b.Err() != nil {
		t.Fatalf("unexpected builder errs: a=%v b=%v", a.Err(), b.Err())
	}
	if a.ID() != 0 || b.ID() != 1 {
		t.Fatalf("ids = %d, %d, want 0, 1", a.ID(), b.ID())
	}
	if s.SystemCount() != 2 {
		t.Fatalf("SystemCount = %d, want 2", s.SystemCount())
	}
}

func TestAddSystemDuplicateRejected(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A"))
	dup := s.AddSystem(newSys("A"))
	if dup.Err() == nil || !errors.Is(dup.Err(), scheduler.ErrDuplicateSystem) {
		t.Fatalf("duplicate err = %v, want ErrDuplicateSystem", dup.Err())
	}
}

func TestAddNilSystemRejected(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	b := s.AddSystem(nil)
	if b.Err() == nil {
		t.Fatal("nil System must be rejected")
	}
}

func TestExplicitBeforeOrdering(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A")).Before("B")
	s.AddSystem(newSys("B"))
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := s.SystemsInOrder()
	if got[0].Name() != "A" || got[1].Name() != "B" {
		t.Fatalf("order = %v, want [A B]", systemNames(got))
	}
}

func TestExplicitAfterOrdering(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A"))
	s.AddSystem(newSys("B")).After("A")
	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := s.SystemsInOrder()
	if got[0].Name() != "A" || got[1].Name() != "B" {
		t.Fatalf("order = %v, want [A B]", systemNames(got))
	}
}

func TestForwardReferenceConstraint(t *testing.T) {
	t.Parallel()

	// Constraint references a system that hasn't been added yet.
	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A")).Before("B")
	s.AddSystem(newSys("B"))
	if err := s.Build(); err != nil {
		t.Fatalf("forward-reference Build err = %v", err)
	}
	got := s.SystemsInOrder()
	if got[0].Name() != "A" || got[1].Name() != "B" {
		t.Fatalf("order = %v, want [A B]", systemNames(got))
	}
}

func TestUnknownSystemConstraintRejected(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A")).Before("Ghost")
	err := s.Build()
	if err == nil || !errors.Is(err, scheduler.ErrUnknownSystem) {
		t.Fatalf("err = %v, want ErrUnknownSystem", err)
	}
}

func TestExplicitCycleRejected(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A")).Before("B")
	s.AddSystem(newSys("B")).Before("A")
	err := s.Build()
	if err == nil || !errors.Is(err, scheduler.ErrScheduleCycle) {
		t.Fatalf("err = %v, want ErrScheduleCycle", err)
	}
}

func TestSelfReferenceRejected(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A")).Before("A")
	err := s.Build()
	if err == nil || !errors.Is(err, scheduler.ErrScheduleCycle) {
		t.Fatalf("self-reference err = %v, want ErrScheduleCycle", err)
	}
}

func TestImplicitAccessConflictAddsEdge(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := registerComponentID[schedPos](t, w)

	s := scheduler.NewSchedule("Update")
	// Reader added first, writer second — implicit edge reader→writer
	// must place reader before writer (registration order tiebreak).
	s.AddSystem(newSysWithReadAccess(t, w, "Reader", posID))
	s.AddSystem(newSysWithWriteAccess(t, w, "Writer", posID))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := s.SystemsInOrder()
	if got[0].Name() != "Reader" || got[1].Name() != "Writer" {
		t.Fatalf("order = %v, want [Reader Writer]", systemNames(got))
	}
	// Verify the implicit edge was actually added.
	if !s.DAG().HasEdge(0, 1) {
		t.Fatal("expected implicit edge 0→1 from Read/Write conflict")
	}
}

func TestImplicitConflictSkippedWhenExplicitEdgeExists(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := registerComponentID[schedPos](t, w)

	s := scheduler.NewSchedule("Update")
	// Two writers of the same component — natural conflict.
	// Caller declares Writer2 BEFORE Writer1 explicitly; implicit
	// pairwise edge in registration order would have been Writer1→Writer2,
	// but the explicit edge wins.
	s.AddSystem(newSysWithWriteAccess(t, w, "Writer1", posID))
	s.AddSystem(newSysWithWriteAccess(t, w, "Writer2", posID)).Before("Writer1")

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := s.SystemsInOrder()
	if got[0].Name() != "Writer2" || got[1].Name() != "Writer1" {
		t.Fatalf("order = %v, want [Writer2 Writer1]", systemNames(got))
	}
}

func TestNoConflictNoEdge(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := registerComponentID[schedPos](t, w)
	velID := registerComponentID[schedVel](t, w)

	s := scheduler.NewSchedule("Update")
	// Two readers of disjoint components — no conflict, no implicit edge.
	s.AddSystem(newSysWithReadAccess(t, w, "ReaderA", posID))
	s.AddSystem(newSysWithReadAccess(t, w, "ReaderB", velID))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if s.DAG().HasEdge(0, 1) || s.DAG().HasEdge(1, 0) {
		t.Fatal("disjoint Read access must not yield implicit edges")
	}
}

func TestReadReadDoesNotAddEdge(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := registerComponentID[schedPos](t, w)

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSysWithReadAccess(t, w, "ReaderA", posID))
	s.AddSystem(newSysWithReadAccess(t, w, "ReaderB", posID))

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	if s.DAG().HasEdge(0, 1) || s.DAG().HasEdge(1, 0) {
		t.Fatal("Read-Read on the same component must not yield an edge")
	}
}

func TestSystemWithoutAccessAwareIsZeroAccess(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	a := s.AddSystem(newSys("A"))
	if a.Err() != nil {
		t.Fatal(a.Err())
	}
	node := s.Node(a.ID())
	if !node.Access().IsEmpty() {
		t.Fatalf("Access = %s, want empty", node.Access())
	}
}

func TestOrderBeforeBuildIsNil(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A"))
	if s.Order() != nil {
		t.Fatal("Order before Build must be nil")
	}
	if s.SystemsInOrder() != nil {
		t.Fatal("SystemsInOrder before Build must be nil")
	}
	if s.DAG() != nil {
		t.Fatal("DAG before Build must be nil")
	}
}

func TestFuncSystemRunCalled(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	called := false
	sys := scheduler.NewFuncSystem("Hello", func(_ *world.World) { called = true })
	sys.Run(w)
	if !called {
		t.Fatal("FuncSystem.Run did not call closure")
	}
}

func TestFuncSystemNilClosureNoOp(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	sys := scheduler.NewFuncSystem("Stub", nil)
	// Must not panic.
	sys.Run(w)
}

func TestFuncSystemAccessRoundTrip(t *testing.T) {
	t.Parallel()

	var a query.Access
	a.AddWrite(7)
	sys := scheduler.NewFuncSystem("Sys", nil).WithAccess(a)
	if got := sys.Access(); !got.Write.Has(7) {
		t.Fatalf("Access().Write = %s, want has(7)", got.Write)
	}

	// Default (no WithAccess) returns the zero Access.
	plain := scheduler.NewFuncSystem("Plain", nil)
	if !plain.Access().IsEmpty() {
		t.Fatal("FuncSystem without WithAccess must have empty Access")
	}
}

func TestComplexScheduleBuild(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := registerComponentID[schedPos](t, w)
	velID := registerComponentID[schedVel](t, w)

	s := scheduler.NewSchedule("Update")
	// Movement reads Pos & Vel, writes Pos (Pos R+W collapses to Write).
	moveAcc := query.Access{}
	moveAcc.AddRead(velID)
	moveAcc.AddWrite(posID)
	s.AddSystem(scheduler.NewFuncSystem("Movement", nil).WithAccess(moveAcc))

	// Spawner writes Pos and Vel — conflicts with Movement on Pos.
	spawnAcc := query.Access{}
	spawnAcc.AddWrite(posID)
	spawnAcc.AddWrite(velID)
	s.AddSystem(scheduler.NewFuncSystem("Spawner", nil).WithAccess(spawnAcc)).Before("Movement")

	// Renderer reads Pos — conflicts with Movement (write).
	renderAcc := query.Access{}
	renderAcc.AddRead(posID)
	s.AddSystem(scheduler.NewFuncSystem("Renderer", nil).WithAccess(renderAcc)).After("Movement")

	if err := s.Build(); err != nil {
		t.Fatal(err)
	}
	got := s.SystemsInOrder()
	want := []string{"Spawner", "Movement", "Renderer"}
	for i, n := range want {
		if got[i].Name() != n {
			t.Fatalf("order[%d] = %s, want %s (full=%v)",
				i, got[i].Name(), n, systemNames(got))
		}
	}
}

// plainSystem is a [scheduler.System] implementation that intentionally
// does NOT satisfy [scheduler.AccessAware], used to exercise the
// fallback-to-zero-Access path.
type plainSystem struct{ name string }

func (p plainSystem) Name() string       { return p.name }
func (p plainSystem) Run(_ *world.World) {}

func TestPlainSystemFallsBackToZeroAccess(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	b := s.AddSystem(plainSystem{name: "Plain"})
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	if !s.Node(b.ID()).Access().IsEmpty() {
		t.Fatal("System without AccessAware must default to zero Access")
	}
}

func TestSystemNodeAccessors(t *testing.T) {
	t.Parallel()

	sys := newSys("X")
	s := scheduler.NewSchedule("Update")
	b := s.AddSystem(sys)
	if b.Err() != nil {
		t.Fatal(b.Err())
	}
	node := s.Node(b.ID())
	if node.ID() != b.ID() {
		t.Fatalf("Node.ID() = %d, want %d", node.ID(), b.ID())
	}
	if node.System() != sys {
		t.Fatal("Node.System() must return the registered system")
	}
}

func TestBuilderErrorPreservedAcrossChaining(t *testing.T) {
	t.Parallel()

	s := scheduler.NewSchedule("Update")
	s.AddSystem(newSys("A"))
	dup := s.AddSystem(newSys("A")).Before("X").After("Y")
	if dup.Err() == nil {
		t.Fatal("duplicate must surface error")
	}
	// Ensure no orphan refs got recorded — Build must succeed without
	// referencing X or Y.
	if err := s.Build(); err != nil {
		t.Fatalf("Build err = %v, want nil", err)
	}
}

// systemNames is a small helper for clearer test failure messages.
func systemNames(systems []scheduler.System) []string {
	out := make([]string, len(systems))
	for i, s := range systems {
		out[i] = s.Name()
	}
	return out
}
