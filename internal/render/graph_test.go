package render

import (
	"errors"
	"testing"

	gpu "github.com/neuengine/neu/pkg/render"
)

func tex(i uint32) gpu.RID { return gpu.MakeRID(gpu.KindTexture, i, 1) }

type fakePass struct {
	name     string
	phase    gpu.RenderPhase
	in, out  []gpu.RID
	recorder *[]string
}

func (p *fakePass) Name() string            { return p.name }
func (p *fakePass) Phase() gpu.RenderPhase   { return p.phase }
func (p *fakePass) Inputs() []gpu.RID        { return p.in }
func (p *fakePass) Outputs() []gpu.RID       { return p.out }
func (p *fakePass) Execute(ctx *PassContext) {
	if p.recorder != nil {
		*p.recorder = append(*p.recorder, p.name)
	}
}

// TestRenderGraph_AcyclicOrder: A(→r1) → B(r1→r2) → C(r2→) sorts A,B,C and
// Execute walks that order.
func TestRenderGraph_AcyclicOrder(t *testing.T) {
	var rec []string
	g := &RenderGraph{}
	g.AddPass(&fakePass{name: "A", out: []gpu.RID{tex(1)}, recorder: &rec})
	g.AddPass(&fakePass{name: "B", in: []gpu.RID{tex(1)}, out: []gpu.RID{tex(2)}, recorder: &rec})
	g.AddPass(&fakePass{name: "C", in: []gpu.RID{tex(2)}, recorder: &rec})

	if err := g.Build(nil); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := g.Order(); !equalInts(got, []int{0, 1, 2}) {
		t.Fatalf("order = %v, want [0 1 2]", got)
	}
	if err := g.Execute(&PassContext{}); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(rec) != 3 || rec[0] != "A" || rec[1] != "B" || rec[2] != "C" {
		t.Fatalf("exec order = %v, want [A B C]", rec)
	}
}

// TestRenderGraph_DeterministicIndependent: independent passes keep ascending
// index order across rebuilds (sorted ready frontier).
func TestRenderGraph_DeterministicIndependent(t *testing.T) {
	g := &RenderGraph{}
	for _, n := range []string{"P0", "P1", "P2", "P3"} {
		g.AddPass(&fakePass{name: n})
	}
	for range 5 {
		if err := g.Build(nil); err != nil {
			t.Fatalf("Build: %v", err)
		}
		if got := g.Order(); !equalInts(got, []int{0, 1, 2, 3}) {
			t.Fatalf("non-deterministic order = %v", got)
		}
	}
}

func TestRenderGraph_Cycle(t *testing.T) {
	g := &RenderGraph{}
	// A reads r2 writes r1; B reads r1 writes r2 → A↔B cycle.
	g.AddPass(&fakePass{name: "A", in: []gpu.RID{tex(2)}, out: []gpu.RID{tex(1)}})
	g.AddPass(&fakePass{name: "B", in: []gpu.RID{tex(1)}, out: []gpu.RID{tex(2)}})

	err := g.Build(nil)
	if !errors.Is(err, ErrRenderGraphCycle) {
		t.Fatalf("Build err = %v, want ErrRenderGraphCycle", err)
	}
	if g.Order() != nil {
		t.Fatal("Order non-nil after cycle (graph must stay unbuilt)")
	}
}

func TestRenderGraph_SelfCycle(t *testing.T) {
	g := &RenderGraph{}
	g.AddPass(&fakePass{name: "Self", in: []gpu.RID{tex(9)}, out: []gpu.RID{tex(9)}})
	if err := g.Build(nil); !errors.Is(err, ErrRenderGraphCycle) {
		t.Fatalf("self-cycle err = %v, want ErrRenderGraphCycle", err)
	}
}

// TestRenderGraph_Barriers pins the golden transition list for a diamond
// A→{B,C}→D over resources r1 (A→B), r2 (A→C), r3 (B→D), r4 (C→D).
func TestRenderGraph_Barriers(t *testing.T) {
	g := &RenderGraph{}
	g.AddPass(&fakePass{name: "A", out: []gpu.RID{tex(1), tex(2)}})           // 0
	g.AddPass(&fakePass{name: "B", in: []gpu.RID{tex(1)}, out: []gpu.RID{tex(3)}}) // 1
	g.AddPass(&fakePass{name: "C", in: []gpu.RID{tex(2)}, out: []gpu.RID{tex(4)}}) // 2
	g.AddPass(&fakePass{name: "D", in: []gpu.RID{tex(3), tex(4)}})            // 3

	if err := g.Build(nil); err != nil {
		t.Fatalf("Build: %v", err)
	}
	got := g.Barriers()
	want := []Barrier{
		{Resource: tex(1), From: 0, To: 1},
		{Resource: tex(2), From: 0, To: 2},
		{Resource: tex(3), From: 1, To: 3},
		{Resource: tex(4), From: 2, To: 3},
	}
	if len(got) != len(want) {
		t.Fatalf("barriers = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("barrier[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// TestRenderGraph_ReleasedExternalInput: a pass reading an external RID with
// zero refs fails Build (INV-3); a transient produced in-graph does not.
func TestRenderGraph_ReleasedExternalInput(t *testing.T) {
	tr := NewResourceTracker(&recBackend{})
	ext := tex(100)

	// (a) never retained → released.
	g := &RenderGraph{}
	g.AddPass(&fakePass{name: "Reader", in: []gpu.RID{ext}})
	if err := g.Build(tr); !errors.Is(err, ErrResourceReleased) {
		t.Fatalf("never-retained input: err = %v, want ErrResourceReleased", err)
	}

	// (b) retained then released (RefCount 0) → still released.
	tr.Retain(ext)
	tr.Release(ext, 0)
	g2 := &RenderGraph{}
	g2.AddPass(&fakePass{name: "Reader", in: []gpu.RID{ext}})
	if err := g2.Build(tr); !errors.Is(err, ErrResourceReleased) {
		t.Fatalf("released input: err = %v, want ErrResourceReleased", err)
	}

	// (c) transient produced by an earlier pass — exempt; Build pins I/O.
	tr3 := NewResourceTracker(&recBackend{})
	transient := tex(7)
	g3 := &RenderGraph{}
	g3.AddPass(&fakePass{name: "Producer", out: []gpu.RID{transient}})
	g3.AddPass(&fakePass{name: "Consumer", in: []gpu.RID{transient}})
	if err := g3.Build(tr3); err != nil {
		t.Fatalf("transient input wrongly rejected: %v", err)
	}
	if tr3.RefCount(transient) == 0 {
		t.Fatal("Build did not pin transient I/O (INV-3)")
	}
}

func TestRenderGraph_ExecuteBeforeBuild(t *testing.T) {
	g := &RenderGraph{}
	g.AddPass(&fakePass{name: "X"})
	if err := g.Execute(&PassContext{}); !errors.Is(err, ErrRenderGraphNotBuilt) {
		t.Fatalf("Execute before Build = %v, want ErrRenderGraphNotBuilt", err)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
