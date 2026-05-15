package view_test

import (
	"slices"
	"testing"

	"github.com/teratron/boltengine/internal/ecs/component"
	"github.com/teratron/boltengine/internal/ecs/entity"
	"github.com/teratron/boltengine/internal/ecs/query"
	"github.com/teratron/boltengine/internal/ecs/view"
	"github.com/teratron/boltengine/internal/ecs/world"
)

// ---- fixtures ---------------------------------------------------------------

type pos struct{ X, Y float32 }
type vel struct{ DX, DY float32 }
type tag struct{}

func collectEntities(it func(yield func(entity.Entity) bool)) []entity.Entity {
	var out []entity.Entity
	for e := range it {
		out = append(out, e)
	}
	return out
}

// ---- View construction ------------------------------------------------------

func TestView_New_InitialScanFindsExisting(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e1 := w.Spawn(component.Data{Value: pos{}})
	e2 := w.Spawn(component.Data{Value: pos{}})
	_ = w.Spawn(component.Data{Value: vel{}}) // no pos -> excluded

	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}

	got := collectEntities(v.Entities(w))
	if len(got) != 2 {
		t.Fatalf("got %d entities, want 2; got=%v", len(got), got)
	}
	want := map[entity.Entity]struct{}{e1: {}, e2: {}}
	for _, e := range got {
		if _, ok := want[e]; !ok {
			t.Fatalf("unexpected entity %v in matched set", e)
		}
	}
}

func TestView_New_EmptyWorld(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	if v.Count(w) != 0 {
		t.Fatalf("Count = %d, want 0 on empty world", v.Count(w))
	}
}

// ---- Push-based subscription ------------------------------------------------

func TestView_AutoTracksNewArchetypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	if v.Count(w) != 0 {
		t.Fatal("view must start empty")
	}

	// Spawn after view creation — listener picks up the new archetype.
	e := w.Spawn(component.Data{Value: pos{X: 1}})
	if v.Count(w) != 1 {
		t.Fatalf("Count = %d, want 1 after spawn", v.Count(w))
	}
	got := collectEntities(v.Entities(w))
	if len(got) != 1 || got[0] != e {
		t.Fatalf("got %v, want [%v]", got, e)
	}
}

func TestView_AutoTracksMultipleArchetypeShapes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := view.TagOf[pos](w)
	v, err := view.Requiring(w, posID)
	if err != nil {
		t.Fatal(err)
	}

	// Three different archetype shapes that all carry pos.
	e1 := w.Spawn(component.Data{Value: pos{}})
	e2 := w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}})
	e3 := w.Spawn(component.Data{Value: pos{}}, component.Data{Value: tag{}})

	if v.MatchedCount() < 3 {
		t.Fatalf("MatchedCount = %d, want ≥3", v.MatchedCount())
	}
	got := collectEntities(v.Entities(w))
	want := map[entity.Entity]struct{}{e1: {}, e2: {}, e3: {}}
	for _, e := range got {
		if _, ok := want[e]; !ok {
			t.Fatalf("unexpected entity %v", e)
		}
	}
	if len(got) != 3 {
		t.Fatalf("got %d entities, want 3", len(got))
	}
}

// ---- Close / unsubscribe ----------------------------------------------------

func TestView_Close_FreezesMatchedList(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	w.Spawn(component.Data{Value: pos{}})
	beforeClose := v.MatchedCount()

	v.Close(w)
	// New archetype with pos+vel — must NOT be added after Close.
	w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}})
	if v.MatchedCount() != beforeClose {
		t.Fatalf("Closed view picked up new archetype: was %d, now %d", beforeClose, v.MatchedCount())
	}
}

func TestView_Close_Idempotent(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	v.Close(w)
	v.Close(w) // must not panic
}

// ---- Contains ---------------------------------------------------------------

func TestView_Contains(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}

	matching := w.Spawn(component.Data{Value: pos{}})
	other := w.Spawn(component.Data{Value: vel{}})

	if !v.Contains(w, matching) {
		t.Fatal("view must contain entities in matched archetypes")
	}
	if v.Contains(w, other) {
		t.Fatal("view must NOT contain entities outside matched archetypes")
	}
}

func TestView_Contains_DeadEntity(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	e := w.Spawn(component.Data{Value: pos{}})
	if !v.Contains(w, e) {
		t.Fatal("view must contain alive matching entity")
	}
	_ = w.Despawn(e)
	if v.Contains(w, e) {
		t.Fatal("Contains must return false for despawned entity")
	}
}

func TestView_Contains_UnspawnedEntity(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	// Reserve via allocator without world record (simulates pre-spawn state).
	e := w.Entities().Allocate()
	if v.Contains(w, e) {
		t.Fatal("Contains must be false for entity with no archetype record")
	}
}

// ---- MatchedArchetypes ------------------------------------------------------

func TestView_MatchedArchetypes_ReturnsCopy(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	w.Spawn(component.Data{Value: pos{}})

	cp := v.MatchedArchetypes()
	if len(cp) != 1 {
		t.Fatalf("MatchedArchetypes len = %d, want 1", len(cp))
	}
	// Mutating the copy must not affect the view.
	cp[0] = 999
	again := v.MatchedArchetypes()
	if again[0] == 999 {
		t.Fatal("MatchedArchetypes must return a defensive copy")
	}
}

// ---- Iteration early-break --------------------------------------------------

func TestView_Entities_StopEarly(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	v, err := view.Requiring(w, view.TagOf[pos](w))
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		w.Spawn(component.Data{Value: pos{}})
	}

	taken := 0
	for range v.Entities(w) {
		taken++
		if taken == 2 {
			break
		}
	}
	if taken != 2 {
		t.Fatalf("early break should yield exactly 2 entities; got %d", taken)
	}
}

// ---- New() with explicit QueryState ----------------------------------------

func TestView_NewWithQueryState_ExcludesArchetypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	posID := view.TagOf[pos](w)
	velID := view.TagOf[vel](w)

	state, err := query.NewQueryState([]component.ID{posID}, []component.ID{velID}, query.Access{})
	if err != nil {
		t.Fatal(err)
	}
	v := view.New(w, state)

	matching := w.Spawn(component.Data{Value: pos{}})
	excluded := w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}})

	got := slices.Collect(v.Entities(w))
	if !slices.Contains(got, matching) {
		t.Fatal("view must contain pos-only entity")
	}
	if slices.Contains(got, excluded) {
		t.Fatal("view must NOT contain entities with excluded vel component")
	}
}

// ---- Tagger -----------------------------------------------------------------

func TestTagOf_RegistersOnFirstUse(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	id1 := view.TagOf[pos](w)
	id2 := view.TagOf[pos](w)
	if id1 == 0 {
		t.Fatal("TagOf must yield a non-zero ID")
	}
	if id1 != id2 {
		t.Fatalf("TagOf must be idempotent; got %d then %d", id1, id2)
	}
}

func TestMaskOf_SingleType(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	id := view.TagOf[pos](w)
	m := view.MaskOf[pos](w)
	if !m.Has(id) {
		t.Fatal("MaskOf must set the bit for T")
	}
	if m.Count() != 1 {
		t.Fatalf("MaskOf count = %d, want 1", m.Count())
	}
}

func TestMaskOf2_TwoTypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	idA := view.TagOf[pos](w)
	idB := view.TagOf[vel](w)
	m := view.MaskOf2[pos, vel](w)
	if !m.Has(idA) || !m.Has(idB) {
		t.Fatal("MaskOf2 must set bits for both T1 and T2")
	}
	if m.Count() != 2 {
		t.Fatalf("MaskOf2 count = %d, want 2", m.Count())
	}
}

func TestMaskOf3_ThreeTypes(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	idA := view.TagOf[pos](w)
	idB := view.TagOf[vel](w)
	idC := view.TagOf[tag](w)
	m := view.MaskOf3[pos, vel, tag](w)
	if !m.Has(idA) || !m.Has(idB) || !m.Has(idC) {
		t.Fatal("MaskOf3 must set bits for all three types")
	}
	if m.Count() != 3 {
		t.Fatalf("MaskOf3 count = %d, want 3", m.Count())
	}
}

func TestMaskOfIDs(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	id1 := view.TagOf[pos](w)
	id2 := view.TagOf[vel](w)
	m := view.MaskOfIDs(id1, id2)
	if !m.Has(id1) || !m.Has(id2) || m.Count() != 2 {
		t.Fatalf("MaskOfIDs failed: has(id1)=%v has(id2)=%v count=%d", m.Has(id1), m.Has(id2), m.Count())
	}
	empty := view.MaskOfIDs()
	if empty.Count() != 0 {
		t.Fatal("MaskOfIDs() with no args must be empty")
	}
}

// ---- ArchetypeStore listeners (direct world tests) --------------------------

func TestArchetypeStore_OnArchetypeCreated(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	var seen []world.ArchetypeID
	id := w.Archetypes().OnArchetypeCreated(func(arch *world.Archetype) {
		seen = append(seen, arch.ID())
	})
	if id == 0 {
		t.Fatal("OnArchetypeCreated must return a non-zero ListenerID")
	}

	w.Spawn(component.Data{Value: pos{}})
	w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}})

	if len(seen) != 2 {
		t.Fatalf("listener invoked %d times, want 2", len(seen))
	}
	// Reusing an existing archetype must NOT re-fire the listener.
	w.Spawn(component.Data{Value: pos{}})
	if len(seen) != 2 {
		t.Fatalf("listener fired on archetype reuse: now %d", len(seen))
	}
}

func TestArchetypeStore_OnArchetypeCreated_NilFnIgnored(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	if id := w.Archetypes().OnArchetypeCreated(nil); id != 0 {
		t.Fatalf("nil listener must yield 0; got %d", id)
	}
}

func TestArchetypeStore_UnregisterListener(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	calls := 0
	id := w.Archetypes().OnArchetypeCreated(func(*world.Archetype) { calls++ })
	w.Spawn(component.Data{Value: pos{}})
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
	w.Archetypes().UnregisterListener(id)
	w.Spawn(component.Data{Value: pos{}}, component.Data{Value: vel{}})
	if calls != 1 {
		t.Fatalf("listener fired after unregister: %d", calls)
	}
	// Idempotent unregister.
	w.Archetypes().UnregisterListener(id)
	w.Archetypes().UnregisterListener(0) // sentinel — no-op
}

// ---- World.ArchetypeOf ------------------------------------------------------

func TestWorld_ArchetypeOf_Live(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Spawn(component.Data{Value: pos{}})
	id, ok := w.ArchetypeOf(e)
	if !ok {
		t.Fatal("ArchetypeOf must succeed for live entity")
	}
	if id == 0 {
		t.Fatal("pos-bearing entity must NOT be in the empty archetype")
	}
}

func TestWorld_ArchetypeOf_Reserved(t *testing.T) {
	t.Parallel()

	w := world.NewWorld()
	e := w.Entities().Allocate()
	if _, ok := w.ArchetypeOf(e); ok {
		t.Fatal("ArchetypeOf must be false for reserved-but-not-spawned entity")
	}
}
