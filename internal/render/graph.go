package render

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	gpu "github.com/neuengine/neu/pkg/render"
)

// ErrRenderGraphCycle is returned by [RenderGraph.Build] when producer→consumer
// edges form a cycle (l1-render-core INV-2). The wrapped error names the passes
// that participate. errors.Is(err, ErrRenderGraphCycle) holds.
var ErrRenderGraphCycle = errors.New("render: graph contains a cycle")

// ErrResourceReleased is returned by [RenderGraph.Build] when a pass reads an
// EXTERNAL resource (one not produced by any pass in the graph) whose
// reference count is zero — i.e. it was released before the graph runs
// (l1-render-core INV-3). Transient resources produced within the graph are
// exempt: Build pins them.
var ErrResourceReleased = errors.New("render: pass references a released RID")

// ErrRenderGraphNotBuilt is returned by [RenderGraph.Execute] before a
// successful [RenderGraph.Build] (mirrors scheduler.ErrScheduleNotBuilt).
var ErrRenderGraphNotBuilt = errors.New("render: graph not built")

// PassContext is the per-frame execution handle passed to [RenderPass.Execute].
// Minimal in 0.1.0 — the full frame context (extracted world, views) is wired
// by T-4A03 (SubApp + 4-phase schedule). [Bootstrap]
type PassContext struct {
	Backend gpu.RenderBackend
	Frame   uint64
}

// RenderPass is a single node in the [RenderGraph]. A pass declares the
// resources it reads and writes BEFORE execution (l1-render-core INV-1) so the
// compiler can order passes, insert barriers, and pin resources.
type RenderPass interface {
	Name() string
	Phase() gpu.RenderPhase
	Inputs() []gpu.RID
	Outputs() []gpu.RID
	Execute(ctx *PassContext)
}

// Barrier records a resource transition between the pass that produces a
// resource and a later pass that consumes it. From/To are indices into the
// built topological order.
type Barrier struct {
	Resource gpu.RID
	From     int
	To       int
}

// RenderGraph is a rebuildable DAG of [RenderPass] nodes. Edges are derived
// from shared resources (producer Outputs → consumer Inputs); the graph is
// topologically sorted by Kahn's algorithm (same pattern as the ECS scheduler
// DAG, C30). Build is idempotent until passes change.
type RenderGraph struct {
	passes   []RenderPass
	order    []int
	barriers []Barrier
	built    bool
}

// AddPass registers p. Invalidates a prior Build.
func (g *RenderGraph) AddPass(p RenderPass) {
	g.passes = append(g.passes, p)
	g.built = false
}

// Build validates resource liveness (INV-3), pins every pass's I/O via tr,
// derives producer→consumer edges, and topologically sorts the graph
// (INV-1/INV-2). A nil tr skips pinning/liveness (pure-ordering use/tests).
// Returns ErrResourceReleased or ErrRenderGraphCycle on violation; the graph
// is left unbuilt on error.
func (g *RenderGraph) Build(tr *ResourceTracker) error {
	g.built = false
	n := len(g.passes)

	// producers[rid] = index of the pass that writes rid (last writer wins;
	// multiple writers of one resource is a separate validation, out of 0.1.0
	// scope — single-writer is the assumed model).
	producers := make(map[gpu.RID]int, n)
	for i, p := range g.passes {
		for _, o := range p.Outputs() {
			if !o.IsNil() {
				producers[o] = i
			}
		}
	}

	// INV-3: an external input (not produced in-graph) with zero refs is
	// released. Validate BEFORE pinning so Build's own Retain can't mask it.
	if tr != nil {
		for _, p := range g.passes {
			for _, in := range p.Inputs() {
				if in.IsNil() {
					continue
				}
				if _, transient := producers[in]; transient {
					continue
				}
				if tr.RefCount(in) == 0 {
					return fmt.Errorf("%w: pass %q reads RID %d",
						ErrResourceReleased, p.Name(), uint64(in))
				}
			}
		}
		// Pin (Retain) all I/O of every pass — alive for the pass duration.
		for _, p := range g.passes {
			for _, r := range p.Inputs() {
				if !r.IsNil() {
					tr.Retain(r)
				}
			}
			for _, r := range p.Outputs() {
				if !r.IsNil() {
					tr.Retain(r)
				}
			}
		}
	}

	// Edges: consumer depends on producer of each non-transient-local input.
	adj := make([][]int, n)
	indeg := make([]int, n)
	seen := make(map[[2]int]struct{})
	for j, p := range g.passes {
		for _, in := range p.Inputs() {
			pi, ok := producers[in]
			if !ok {
				continue
			}
			if pi == j {
				// A pass reading its own output is a self-cycle (INV-2),
				// matching the scheduler DAG's from==to rejection.
				return cycleError([]string{g.passes[j].Name()})
			}
			e := [2]int{pi, j}
			if _, dup := seen[e]; dup {
				continue
			}
			seen[e] = struct{}{}
			adj[pi] = append(adj[pi], j)
			indeg[j]++
		}
	}
	for i := range adj {
		slices.Sort(adj[i])
	}

	// Kahn with a sorted ready frontier → deterministic order on ties.
	ready := make([]int, 0, n)
	for i := range n {
		if indeg[i] == 0 {
			ready = append(ready, i)
		}
	}
	order := make([]int, 0, n)
	for len(ready) > 0 {
		next := ready[0]
		ready = ready[1:]
		order = append(order, next)
		for _, dst := range adj[next] {
			indeg[dst]--
			if indeg[dst] == 0 {
				pos, _ := slices.BinarySearch(ready, dst)
				ready = slices.Insert(ready, pos, dst)
			}
		}
	}
	if len(order) != n {
		names := make([]string, 0, n-len(order))
		for i, d := range indeg {
			if d > 0 {
				names = append(names, g.passes[i].Name())
			}
		}
		return cycleError(names)
	}

	// Barriers: producer→consumer transition per shared resource, in order.
	pos := make([]int, n) // pass index → position in order
	for p, idx := range order {
		pos[idx] = p
	}
	var barriers []Barrier
	for j, p := range g.passes {
		for _, in := range p.Inputs() {
			if pi, ok := producers[in]; ok && pi != j {
				barriers = append(barriers, Barrier{Resource: in, From: pos[pi], To: pos[j]})
			}
		}
	}
	slices.SortFunc(barriers, func(a, b Barrier) int {
		if a.From != b.From {
			return a.From - b.From
		}
		if a.To != b.To {
			return a.To - b.To
		}
		return int(a.Resource) - int(b.Resource)
	})

	g.order = order
	g.barriers = barriers
	g.built = true
	return nil
}

// Execute walks the built topological order, invoking each pass. Returns
// ErrRenderGraphNotBuilt if Build has not succeeded.
func (g *RenderGraph) Execute(ctx *PassContext) error {
	if !g.built {
		return ErrRenderGraphNotBuilt
	}
	for _, idx := range g.order {
		g.passes[idx].Execute(ctx)
	}
	return nil
}

// Order returns the built execution order as pass indices (nil if unbuilt).
func (g *RenderGraph) Order() []int {
	if !g.built {
		return nil
	}
	return slices.Clone(g.order)
}

// Barriers returns the resource transitions inserted by Build (nil if unbuilt).
func (g *RenderGraph) Barriers() []Barrier {
	if !g.built {
		return nil
	}
	return slices.Clone(g.barriers)
}

func cycleError(passes []string) error { return &graphCycleError{passes: passes} }

type graphCycleError struct{ passes []string }

func (e *graphCycleError) Error() string {
	if len(e.passes) == 0 {
		return ErrRenderGraphCycle.Error()
	}
	return ErrRenderGraphCycle.Error() + " (passes: " + strings.Join(e.passes, ", ") + ")"
}

func (e *graphCycleError) Unwrap() error { return ErrRenderGraphCycle }
