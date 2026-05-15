package scheduler

import (
	"errors"
	"slices"
)

// SystemNodeID indexes a node within a [DAG]. IDs are dense, contiguous
// integers assigned in the order [Schedule.AddSystem] is called.
type SystemNodeID uint32

// ErrScheduleCycle is returned by [DAG.Build] (and therefore by
// [Schedule.Build]) when the constructed graph contains a cycle. The
// error string includes the node IDs that participate in the cycle as
// reported by Kahn's algorithm.
var ErrScheduleCycle = errors.New("ecs: cycle detected in schedule DAG")

// DAG is a directed acyclic graph of [SystemNodeID]s used to resolve
// execution order. Edges encode "from must run before to" semantics; the
// final topological order is produced by [DAG.Build].
//
// The graph is intentionally minimal — it knows nothing about systems,
// access metadata, or run conditions. Higher layers ([Schedule]) translate
// scheduler concerns into edges and read back the order.
type DAG struct {
	edges     map[edge]struct{}
	adj       [][]SystemNodeID
	inDegree  []int
	sorted    []SystemNodeID
	nodeCount int
	built     bool
}

// edge is the canonical form of a "from before to" relationship. Stored as
// a map key so duplicate AddEdge calls collapse silently.
type edge struct {
	from SystemNodeID
	to   SystemNodeID
}

// NewDAG returns a DAG sized for nodeCount systems. SystemNodeIDs in the
// range [0, nodeCount) are valid; AddEdge with out-of-range endpoints
// panics.
func NewDAG(nodeCount int) *DAG {
	return &DAG{
		nodeCount: nodeCount,
		edges:     make(map[edge]struct{}),
	}
}

// NodeCount returns the number of nodes registered with the DAG.
func (d *DAG) NodeCount() int { return d.nodeCount }

// AddEdge declares that from must run before to. Duplicate edges are
// folded; self-loops (from == to) are rejected as cycles at [DAG.Build]
// time. Panics if either endpoint is out of range.
func (d *DAG) AddEdge(from, to SystemNodeID) {
	if int(from) >= d.nodeCount || int(to) >= d.nodeCount {
		panic("scheduler.DAG.AddEdge: node id out of range")
	}
	d.edges[edge{from: from, to: to}] = struct{}{}
	d.built = false
}

// HasEdge reports whether (from → to) was declared.
func (d *DAG) HasEdge(from, to SystemNodeID) bool {
	_, ok := d.edges[edge{from: from, to: to}]
	return ok
}

// Build runs Kahn's algorithm to topologically sort the graph.
//
//   - Computes in-degrees for every node.
//   - Repeatedly emits nodes with zero in-degree (lowest ID first, for
//     deterministic ordering across runs) and decrements the in-degree of
//     their successors.
//   - When the emitted count is less than the node count, the remaining
//     nodes form one or more cycles; the wrapped error names them.
//
// Build is idempotent: calling it twice without intervening edge changes
// returns the same order; modifying the graph after Build resets `built`
// so the next call recomputes.
func (d *DAG) Build() error {
	d.adj = make([][]SystemNodeID, d.nodeCount)
	d.inDegree = make([]int, d.nodeCount)

	for e := range d.edges {
		if e.from == e.to {
			return cycleError([]SystemNodeID{e.from})
		}
		d.adj[e.from] = append(d.adj[e.from], e.to)
		d.inDegree[e.to]++
	}
	for i := range d.adj {
		slices.Sort(d.adj[i])
	}

	// Use a sorted slice as the "ready" frontier so output ordering is
	// deterministic for nodes that share an in-degree of zero.
	ready := make([]SystemNodeID, 0, d.nodeCount)
	for i := 0; i < d.nodeCount; i++ {
		if d.inDegree[i] == 0 {
			ready = append(ready, SystemNodeID(i))
		}
	}

	d.sorted = make([]SystemNodeID, 0, d.nodeCount)
	for len(ready) > 0 {
		// Pop smallest ID for stable order.
		next := ready[0]
		ready = ready[1:]
		d.sorted = append(d.sorted, next)
		for _, dst := range d.adj[next] {
			d.inDegree[dst]--
			if d.inDegree[dst] == 0 {
				// Insert keeping ready sorted ascending.
				i, _ := slices.BinarySearch(ready, dst)
				ready = slices.Insert(ready, i, dst)
			}
		}
	}

	if len(d.sorted) != d.nodeCount {
		// Collect the residual cycle members (any node with inDegree > 0).
		cycle := make([]SystemNodeID, 0, d.nodeCount-len(d.sorted))
		for i, deg := range d.inDegree {
			if deg > 0 {
				cycle = append(cycle, SystemNodeID(i))
			}
		}
		return cycleError(cycle)
	}

	d.built = true
	return nil
}

// TopologicalOrder returns the sorted execution order. Must only be called
// after [DAG.Build] has completed without error.
func (d *DAG) TopologicalOrder() []SystemNodeID {
	if !d.built {
		return nil
	}
	out := make([]SystemNodeID, len(d.sorted))
	copy(out, d.sorted)
	return out
}

// cycleError wraps [ErrScheduleCycle] with the offending node list.
func cycleError(nodes []SystemNodeID) error {
	return &dagCycleError{nodes: nodes}
}

type dagCycleError struct {
	nodes []SystemNodeID
}

func (e *dagCycleError) Error() string {
	if len(e.nodes) == 0 {
		return ErrScheduleCycle.Error()
	}
	out := ErrScheduleCycle.Error() + " (nodes: "
	for i, n := range e.nodes {
		if i > 0 {
			out += ", "
		}
		out += systemNodeIDString(n)
	}
	return out + ")"
}

func (e *dagCycleError) Unwrap() error { return ErrScheduleCycle }

func systemNodeIDString(n SystemNodeID) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	v := uint32(n)
	var buf [10]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = digits[v%10]
		v /= 10
	}
	return string(buf[i:])
}
