package hierarchy

import (
	"reflect"
	"slices"
	"testing"

	"github.com/neuengine/neu/internal/ecs/command"
	"github.com/neuengine/neu/internal/ecs/component"
	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/scheduler"
	"github.com/neuengine/neu/internal/ecs/world"
	"github.com/neuengine/neu/pkg/app/appface"
	neu "github.com/neuengine/neu/pkg/math"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newWorld() *world.World { return world.NewWorld() }

func spawn(w *world.World) entity.Entity {
	return w.SpawnEmpty()
}

// addChildDirect calls addChildImmediate directly (skipping command queue).
func addChildDirect(w *world.World, parent, child entity.Entity) {
	addChildImmediate(w, parent, child)
}

// removeParentDirect calls removeParentImmediate directly.
func removeParentDirect(w *world.World, child entity.Entity) {
	removeParentImmediate(w, child)
}

// ── ChildOf / Children maintenance ───────────────────────────────────────────

func TestAddChild_BasicLink(t *testing.T) {
	w := newWorld()
	parent := spawn(w)
	child := spawn(w)

	addChildDirect(w, parent, child)

	co, ok := world.Get[ChildOf](w, child)
	if !ok {
		t.Fatal("child missing ChildOf")
	}
	if co.Parent != parent {
		t.Fatalf("ChildOf.Parent = %v, want %v", co.Parent, parent)
	}

	ch, ok := world.Get[Children](w, parent)
	if !ok {
		t.Fatal("parent missing Children")
	}
	if ch.Len() != 1 || !ch.Contains(child) {
		t.Fatalf("Children = %v, want [%v]", ch.Slice(), child)
	}
}

func TestAddChild_MultipleChildren(t *testing.T) {
	w := newWorld()
	parent := spawn(w)
	c1, c2, c3 := spawn(w), spawn(w), spawn(w)

	addChildDirect(w, parent, c1)
	addChildDirect(w, parent, c2)
	addChildDirect(w, parent, c3)

	ch, _ := world.Get[Children](w, parent)
	if ch.Len() != 3 {
		t.Fatalf("Children.Len = %d, want 3", ch.Len())
	}
	for _, c := range []entity.Entity{c1, c2, c3} {
		if !ch.Contains(c) {
			t.Errorf("Children missing %v", c)
		}
	}
}

func TestRemoveParent_UnlinksChild(t *testing.T) {
	w := newWorld()
	parent := spawn(w)
	child := spawn(w)
	addChildDirect(w, parent, child)

	removeParentDirect(w, child)

	if _, ok := world.Get[ChildOf](w, child); ok {
		t.Fatal("child still has ChildOf after RemoveParent")
	}
	ch, ok := world.Get[Children](w, parent)
	if ok && ch.Contains(child) {
		t.Fatal("parent Children still contains child after RemoveParent")
	}
}

func TestAddChild_SelfParentRejected(t *testing.T) {
	w := newWorld()
	e := spawn(w)
	addChildDirect(w, e, e) // self-parenting must be silently rejected
	if _, ok := world.Get[ChildOf](w, e); ok {
		t.Fatal("self-parenting was not rejected")
	}
}

func TestAddChild_CycleDetection(t *testing.T) {
	w := newWorld()
	a, b, c := spawn(w), spawn(w), spawn(w)
	addChildDirect(w, a, b) // a → b
	addChildDirect(w, b, c) // b → c
	addChildDirect(w, c, a) // c → a would create a → b → c → a cycle

	// a must still have no parent
	if _, ok := world.Get[ChildOf](w, a); ok {
		t.Fatal("cycle was not rejected: a has a parent after c→a attempt")
	}
}

func TestAddChild_Reparent(t *testing.T) {
	w := newWorld()
	p1, p2, child := spawn(w), spawn(w), spawn(w)
	addChildDirect(w, p1, child)
	addChildDirect(w, p2, child) // reparent: child moves from p1 to p2

	co, _ := world.Get[ChildOf](w, child)
	if co.Parent != p2 {
		t.Fatalf("child.Parent = %v, want p2 %v", co.Parent, p2)
	}

	// p1 must no longer list child
	if ch, ok := world.Get[Children](w, p1); ok && ch.Contains(child) {
		t.Fatal("p1 still lists child after reparent")
	}

	// p2 must list child
	ch2, _ := world.Get[Children](w, p2)
	if !ch2.Contains(child) {
		t.Fatal("p2 does not list child after reparent")
	}
}

// ── Traversal ─────────────────────────────────────────────────────────────────

func buildTree(w *world.World) (root, mid, leaf1, leaf2 entity.Entity) {
	root = spawn(w)
	mid = spawn(w)
	leaf1 = spawn(w)
	leaf2 = spawn(w)
	addChildDirect(w, root, mid)
	addChildDirect(w, mid, leaf1)
	addChildDirect(w, mid, leaf2)
	return
}

func TestChildrenOf(t *testing.T) {
	w := newWorld()
	root, mid, _, _ := buildTree(w)

	var got []entity.Entity
	for e := range ChildrenOf(w, root) {
		got = append(got, e)
	}
	if len(got) != 1 || got[0] != mid {
		t.Fatalf("ChildrenOf(root) = %v, want [mid]", got)
	}
}

func TestDescendants(t *testing.T) {
	w := newWorld()
	root, mid, leaf1, leaf2 := buildTree(w)

	var got []entity.Entity
	for e := range Descendants(w, root) {
		got = append(got, e)
	}
	if len(got) != 3 {
		t.Fatalf("Descendants count = %d, want 3", len(got))
	}
	inSet := func(e entity.Entity) bool {
		return slices.Contains(got, e)
	}
	for _, e := range []entity.Entity{mid, leaf1, leaf2} {
		if !inSet(e) {
			t.Errorf("Descendants missing %v", e)
		}
	}
}

func TestAncestors(t *testing.T) {
	w := newWorld()
	root, mid, leaf1, _ := buildTree(w)

	var got []entity.Entity
	for e := range Ancestors(w, leaf1) {
		got = append(got, e)
	}
	if len(got) != 2 || got[0] != mid || got[1] != root {
		t.Fatalf("Ancestors(leaf1) = %v, want [mid, root]", got)
	}
}

func TestRoot(t *testing.T) {
	w := newWorld()
	root, _, leaf1, _ := buildTree(w)

	if got := Root(w, leaf1); got != root {
		t.Fatalf("Root(leaf1) = %v, want root %v", got, root)
	}
	if got := Root(w, root); got != root {
		t.Fatalf("Root(root) = %v, want root (self)", got)
	}
}

func TestIsDescendantOf(t *testing.T) {
	w := newWorld()
	root, mid, leaf1, leaf2 := buildTree(w)

	if !IsDescendantOf(w, leaf1, root) {
		t.Error("leaf1 should be descendant of root")
	}
	if !IsDescendantOf(w, mid, root) {
		t.Error("mid should be descendant of root")
	}
	if IsDescendantOf(w, root, leaf1) {
		t.Error("root should NOT be descendant of leaf1")
	}
	if IsDescendantOf(w, leaf1, leaf2) {
		t.Error("leaf1 should NOT be descendant of leaf2")
	}
}

// ── DespawnRecursive ──────────────────────────────────────────────────────────

func TestDespawnRecursive(t *testing.T) {
	w := newWorld()
	root, mid, leaf1, leaf2 := buildTree(w)

	despawnRecursiveImmediate(w, root)

	for _, e := range []entity.Entity{root, mid, leaf1, leaf2} {
		if w.Contains(e) {
			t.Errorf("entity %v still alive after DespawnRecursive", e)
		}
	}
}

// ── Transform / GlobalTransform ───────────────────────────────────────────────

func TestNewTransform_Identity(t *testing.T) {
	tr := NewTransform()
	if tr.Translation != (neu.Vec3{}) {
		t.Error("NewTransform: non-zero translation")
	}
	if tr.Rotation != neu.QuatIdentity() {
		t.Error("NewTransform: non-identity rotation")
	}
	if tr.Scale != (neu.Vec3{X: 1, Y: 1, Z: 1}) {
		t.Error("NewTransform: non-unit scale")
	}
}

func TestFromTranslation(t *testing.T) {
	v := neu.Vec3{X: 1, Y: 2, Z: 3}
	tr := FromTranslation(v)
	if tr.Translation != v {
		t.Errorf("FromTranslation.Translation = %v, want %v", tr.Translation, v)
	}
	if tr.Rotation != neu.QuatIdentity() {
		t.Error("FromTranslation: non-identity rotation")
	}
}

func TestFromRotation(t *testing.T) {
	q := neu.QuatFromAxisAngle(neu.Vec3{X: 0, Y: 1, Z: 0}, 1.0)
	tr := FromRotation(q)
	if tr.Rotation != q {
		t.Errorf("FromRotation.Rotation = %v, want %v", tr.Rotation, q)
	}
	if tr.Translation != (neu.Vec3{}) {
		t.Error("FromRotation: non-zero translation")
	}
}

func TestTransformToAffine3A_Identity(t *testing.T) {
	m := NewTransform().ToAffine3A()
	id := neu.Affine3AIdentity()
	if m != id {
		t.Errorf("identity Transform.ToAffine3A = %v, want identity %v", m, id)
	}
}

func TestGlobalTransformMethods(t *testing.T) {
	gt := NewGlobalTransform()
	if gt.Translation() != (neu.Vec3{}) {
		t.Error("NewGlobalTransform: non-zero translation")
	}
	if gt.Right() != (neu.Vec3{X: 1}) {
		t.Errorf("Right = %v, want {1,0,0}", gt.Right())
	}
	if gt.Up() != (neu.Vec3{Y: 1}) {
		t.Errorf("Up = %v, want {0,1,0}", gt.Up())
	}
	fwd := gt.Forward()
	if fwd != (neu.Vec3{Z: -1}) {
		t.Errorf("Forward = %v, want {0,0,-1}", fwd)
	}
}

func TestPropagateTransforms_ThreeLevel(t *testing.T) {
	w := newWorld()

	// Build 3-level hierarchy: root(0,0,0) → mid(1,0,0) → leaf(0,1,0)
	rootE := w.Spawn(component.Data{Value: NewTransform()})
	midE := w.Spawn(component.Data{Value: FromTranslation(neu.Vec3{X: 1})})
	leafE := w.Spawn(component.Data{Value: FromTranslation(neu.Vec3{Y: 1})})

	addChildDirect(w, rootE, midE)
	addChildDirect(w, midE, leafE)

	propagateTransforms(w)

	// root GlobalTransform: world pos = {0,0,0}
	rootGT, _ := world.Get[GlobalTransform](w, rootE)
	if rootGT.Translation() != (neu.Vec3{}) {
		t.Errorf("root world pos = %v, want {0,0,0}", rootGT.Translation())
	}

	// mid GlobalTransform: world pos = {1,0,0}
	midGT, _ := world.Get[GlobalTransform](w, midE)
	wantMid := neu.Vec3{X: 1}
	if midGT.Translation() != wantMid {
		t.Errorf("mid world pos = %v, want %v", midGT.Translation(), wantMid)
	}

	// leaf GlobalTransform: world pos = {1,1,0}
	leafGT, _ := world.Get[GlobalTransform](w, leafE)
	wantLeaf := neu.Vec3{X: 1, Y: 1}
	if leafGT.Translation() != wantLeaf {
		t.Errorf("leaf world pos = %v, want %v", leafGT.Translation(), wantLeaf)
	}
}

func TestPropagateTransforms_UpdateAfterMove(t *testing.T) {
	w := newWorld()
	rootE := w.Spawn(component.Data{Value: NewTransform()})
	childE := w.Spawn(component.Data{Value: FromTranslation(neu.Vec3{X: 2})})
	addChildDirect(w, rootE, childE)

	propagateTransforms(w)

	// Move root to {5,0,0}
	rootTR, _ := world.Get[Transform](w, rootE)
	rootTR.Translation = neu.Vec3{X: 5}

	propagateTransforms(w)

	childGT, _ := world.Get[GlobalTransform](w, childE)
	want := neu.Vec3{X: 7}
	if childGT.Translation() != want {
		t.Errorf("child world pos after root move = %v, want %v", childGT.Translation(), want)
	}
}

// ── Fuzz: random reparent/despawn must never create cycles ───────────────────

// ── Children.Slice ────────────────────────────────────────────────────────────

func TestChildren_Slice(t *testing.T) {
	w := newWorld()
	parent := spawn(w)
	c1, c2 := spawn(w), spawn(w)
	addChildDirect(w, parent, c1)
	addChildDirect(w, parent, c2)

	ch, _ := world.Get[Children](w, parent)
	s := ch.Slice()
	if !slices.Contains(s, c1) || !slices.Contains(s, c2) {
		t.Fatalf("Slice = %v, want [c1, c2]", s)
	}
	// Verify it is a copy — mutating s must not affect Children.
	s[0] = entity.Entity{}
	ch2, _ := world.Get[Children](w, parent)
	if ch2.Len() != 2 {
		t.Error("Slice returned a reference, not a copy")
	}
}

// ── Command-queue public API ──────────────────────────────────────────────────

func makeCommands(w *world.World) (*command.CommandBuffer, *command.Commands) {
	buf := command.NewCommandBuffer(w.Entities(), 0)
	return buf, command.NewCommands(buf)
}

func TestAddChild_CommandAPI(t *testing.T) {
	w := newWorld()
	parent, child := spawn(w), spawn(w)

	buf, cmds := makeCommands(w)
	AddChild(cmds, parent, child)
	buf.Apply(w)

	co, ok := world.Get[ChildOf](w, child)
	if !ok || co.Parent != parent {
		t.Fatal("AddChild via command did not link child")
	}
}

func TestSetParent_CommandAPI(t *testing.T) {
	w := newWorld()
	p1, p2, child := spawn(w), spawn(w), spawn(w)
	addChildDirect(w, p1, child)

	buf, cmds := makeCommands(w)
	SetParent(cmds, child, p2)
	buf.Apply(w)

	co, _ := world.Get[ChildOf](w, child)
	if co.Parent != p2 {
		t.Fatalf("SetParent: child.Parent = %v, want p2", co.Parent)
	}
}

func TestRemoveParent_CommandAPI(t *testing.T) {
	w := newWorld()
	parent, child := spawn(w), spawn(w)
	addChildDirect(w, parent, child)

	buf, cmds := makeCommands(w)
	RemoveParent(cmds, child)
	buf.Apply(w)

	if _, ok := world.Get[ChildOf](w, child); ok {
		t.Fatal("RemoveParent via command left ChildOf on child")
	}
}

func TestDespawnRecursive_CommandAPI(t *testing.T) {
	w := newWorld()
	root, mid, leaf := spawn(w), spawn(w), spawn(w)
	addChildDirect(w, root, mid)
	addChildDirect(w, mid, leaf)

	buf, cmds := makeCommands(w)
	DespawnRecursive(cmds, root)
	buf.Apply(w)

	for _, e := range []entity.Entity{root, mid, leaf} {
		if w.Contains(e) {
			t.Errorf("entity %v still alive after DespawnRecursive", e)
		}
	}
}

// ── Dead entity edge cases ────────────────────────────────────────────────────

func TestAddChild_DeadParentIgnored(t *testing.T) {
	w := newWorld()
	parent, child := spawn(w), spawn(w)
	_ = w.Despawn(parent)
	addChildDirect(w, parent, child) // must be a no-op
	if _, ok := world.Get[ChildOf](w, child); ok {
		t.Fatal("ChildOf inserted with dead parent")
	}
}

func TestAddChild_DeadChildIgnored(t *testing.T) {
	w := newWorld()
	parent, child := spawn(w), spawn(w)
	_ = w.Despawn(child)
	addChildDirect(w, parent, child) // must be a no-op
	if _, ok := world.Get[Children](w, parent); ok {
		t.Fatal("Children inserted with dead child")
	}
}

// ── Early-exit iterator coverage ─────────────────────────────────────────────

func TestChildrenOf_EarlyExit(t *testing.T) {
	w := newWorld()
	parent := spawn(w)
	c1, c2 := spawn(w), spawn(w)
	addChildDirect(w, parent, c1)
	addChildDirect(w, parent, c2)

	count := 0
	for range ChildrenOf(w, parent) {
		count++
		break // stop after first
	}
	if count != 1 {
		t.Fatalf("expected early exit after 1, got %d", count)
	}
}

func TestChildrenOf_NoChildren(t *testing.T) {
	w := newWorld()
	e := spawn(w)
	count := 0
	for range ChildrenOf(w, e) {
		count++
	}
	if count != 0 {
		t.Fatalf("expected 0 iterations on entity with no Children, got %d", count)
	}
}

func TestDescendants_EarlyExit(t *testing.T) {
	w := newWorld()
	root, mid, leaf := spawn(w), spawn(w), spawn(w)
	addChildDirect(w, root, mid)
	addChildDirect(w, mid, leaf)

	count := 0
	for range Descendants(w, root) {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("expected early exit after 1 descendant, got %d", count)
	}
}

func TestAncestors_EarlyExit(t *testing.T) {
	w := newWorld()
	root, mid, leaf := spawn(w), spawn(w), spawn(w)
	addChildDirect(w, root, mid)
	addChildDirect(w, mid, leaf)

	count := 0
	for range Ancestors(w, leaf) {
		count++
		break
	}
	if count != 1 {
		t.Fatalf("expected early exit after 1 ancestor, got %d", count)
	}
}

// ── Transform extras ──────────────────────────────────────────────────────────

func TestLookAt(t *testing.T) {
	eye := neu.Vec3{X: 0, Y: 0, Z: 5}
	target := neu.Vec3{}
	up := neu.Vec3{Y: 1}
	tr := LookAt(eye, target, up)
	if tr.Translation != eye {
		t.Errorf("LookAt.Translation = %v, want %v", tr.Translation, eye)
	}
	// Rotation must not be the zero value.
	if tr.Rotation == (neu.Quat{}) {
		t.Error("LookAt.Rotation is zero")
	}
}

func TestFromAffine3A_RoundTrip(t *testing.T) {
	id := neu.Affine3AIdentity()
	gt := FromAffine3A(id)
	if gt.Affine3A() != id {
		t.Errorf("FromAffine3A roundtrip: got %v, want %v", gt.Affine3A(), id)
	}
}

func TestGlobalTransform_Mul(t *testing.T) {
	offset := neu.Vec3{X: 3}
	parent := FromAffine3A(neu.FromTRS(offset, neu.QuatIdentity(), neu.Vec3{X: 1, Y: 1, Z: 1}))
	child := NewGlobalTransform()
	combined := parent.Mul(child)
	// translation of identity child in parent frame = parent translation
	if combined.Translation() != offset {
		t.Errorf("Mul translation = %v, want %v", combined.Translation(), offset)
	}
}

// ── Plugin ────────────────────────────────────────────────────────────────────

type mockHierBuilder struct {
	w       *world.World
	systems []string
}

func (m *mockHierBuilder) World() *world.World { return m.w }
func (m *mockHierBuilder) AddSystem(sched string, sys scheduler.System) appface.Builder {
	m.systems = append(m.systems, sched+":"+sys.Name())
	return m
}
func (m *mockHierBuilder) AddSystems(sched string, systems ...scheduler.System) appface.Builder {
	for _, s := range systems {
		m.AddSystem(sched, s)
	}
	return m
}
func (m *mockHierBuilder) SetResource(v any) appface.Builder  { return m }
func (m *mockHierBuilder) InitResource(v any) appface.Builder { return m }
func (m *mockHierBuilder) AddPlugin(p appface.Plugin) appface.Builder {
	p.Build(m)
	return m
}
func (m *mockHierBuilder) AddPlugins(g appface.PluginGroup) appface.Builder { return m }

func TestHierarchyPlugin_Build(t *testing.T) {
	w := newWorld()
	mb := &mockHierBuilder{w: w}
	HierarchyPlugin{}.Build(mb)

	// Plugin must register the propagation system in PostUpdate.
	found := slices.Contains(mb.systems, appface.PostUpdate+":hierarchy.PropagateTransforms")
	if !found {
		t.Errorf("HierarchyPlugin did not register propagation system; systems = %v", mb.systems)
	}

	// Components must be pre-registered.
	reg := w.Components()
	types := []struct {
		name string
		id   func() bool
	}{
		{"ChildOf", func() bool { _, ok := reg.Lookup(reflect.TypeFor[ChildOf]()); return ok }},
		{"Children", func() bool { _, ok := reg.Lookup(reflect.TypeFor[Children]()); return ok }},
		{"Transform", func() bool { _, ok := reg.Lookup(reflect.TypeFor[Transform]()); return ok }},
		{"GlobalTransform", func() bool { _, ok := reg.Lookup(reflect.TypeFor[GlobalTransform]()); return ok }},
	}
	for _, tc := range types {
		if !tc.id() {
			t.Errorf("component %q not registered after HierarchyPlugin.Build", tc.name)
		}
	}
}

// ── Fuzz ──────────────────────────────────────────────────────────────────────

func FuzzHierarchyReparent(f *testing.F) {
	f.Add(uint8(3), uint8(0), uint8(1), uint8(2), uint8(1), uint8(0))

	f.Fuzz(func(t *testing.T, a, b, c, d, e, g uint8) {
		w := newWorld()
		const n = 4
		entities := make([]entity.Entity, n)
		for i := range entities {
			entities[i] = spawn(w)
		}

		ops := []struct{ p, c uint8 }{{a % n, b % n}, {c % n, d % n}, {e % n, g % n}}
		for _, op := range ops {
			parent := entities[op.p]
			child := entities[op.c]
			if w.Contains(parent) && w.Contains(child) {
				addChildDirect(w, parent, child)
			}
		}

		// Invariant: no entity is its own ancestor.
		for _, ent := range entities {
			if !w.Contains(ent) {
				continue
			}
			for anc := range Ancestors(w, ent) {
				if anc == ent {
					t.Fatalf("cycle detected: entity %v is its own ancestor", ent)
				}
			}
		}
	})
}
