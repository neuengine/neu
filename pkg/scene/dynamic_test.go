package scene

import (
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
	"github.com/neuengine/neu/internal/ecs/typereg"
)

// ─── in-memory mock World ────────────────────────────────────────────────────

// mockArch represents one archetype snapshot.
type mockArch struct {
	entities []entity.Entity
	rows     []map[string]any
}

func (a *mockArch) Entities() []entity.Entity { return a.entities }
func (a *mockArch) ComponentValues(row int) map[string]any {
	if row < 0 || row >= len(a.rows) {
		return nil
	}
	return a.rows[row]
}

// mockWorldReader implements WorldReader backed by a slice of mockArch.
type mockWorldReader struct {
	archs []*mockArch
}

func (m *mockWorldReader) EachArchetype(fn func(ArchetypeView) bool) {
	for _, a := range m.archs {
		if !fn(a) {
			return
		}
	}
}

// mockWorldWriter implements WorldWriter and records spawned components.
type mockWorldWriter struct {
	nextIdx    uint32
	nextGen    uint32
	components map[entity.EntityID][]any // entity → component values
}

func newMockWorldWriter() *mockWorldWriter {
	return &mockWorldWriter{
		nextIdx:    1,
		components: make(map[entity.EntityID][]any),
	}
}

func (m *mockWorldWriter) SpawnEmpty() entity.Entity {
	e := entity.NewEntity(m.nextIdx, m.nextGen)
	m.nextIdx++
	m.components[e.ID()] = nil
	return e
}

func (m *mockWorldWriter) InsertComponent(e entity.Entity, value any) error {
	m.components[e.ID()] = append(m.components[e.ID()], value)
	return nil
}

// ─── test types ──────────────────────────────────────────────────────────────

type Position struct{ X, Y, Z float32 }
type Velocity struct{ DX, DY float32 }
type ChildOf struct{ Parent entity.EntityID }

// ─── helpers ─────────────────────────────────────────────────────────────────

func buildTestRegistry() *typereg.TypeRegistry {
	reg := typereg.NewTypeRegistry()
	typereg.RegisterType[Position](reg)
	typereg.RegisterType[Velocity](reg)
	typereg.RegisterType[ChildOf](reg)
	return reg
}

func makeEntity(idx uint32) entity.Entity {
	return entity.NewEntity(idx, 1)
}

// ─── DynamicSceneBuilder tests ───────────────────────────────────────────────

func TestDynamicSceneBuilderBasic(t *testing.T) {
	reg := buildTestRegistry()

	e1 := makeEntity(1)
	e2 := makeEntity(2)

	posReg := reg.ResolveByName(reg.ResolveByID(1).Name) // Position TypeID=1
	_ = posReg

	worldReader := &mockWorldReader{
		archs: []*mockArch{
			{
				entities: []entity.Entity{e1, e2},
				rows: []map[string]any{
					{"github.com/neuengine/neu/pkg/scene.Position": Position{X: 1, Y: 2, Z: 3}},
					{"github.com/neuengine/neu/pkg/scene.Position": Position{X: 4, Y: 5, Z: 6}},
				},
			},
		},
	}

	sc := NewDynamicSceneBuilder(worldReader, reg).Build()
	if len(sc.Entities()) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(sc.Entities()))
	}
}

func TestDynamicSceneBuilderFilter(t *testing.T) {
	reg := buildTestRegistry()
	e1 := makeEntity(1)

	// Two components: Position and Velocity. Filter denies Velocity.
	worldReader := &mockWorldReader{
		archs: []*mockArch{
			{
				entities: []entity.Entity{e1},
				rows: []map[string]any{
					{
						typeName(Position{}): Position{X: 1},
						typeName(Velocity{}): Velocity{DX: 5},
					},
				},
			},
		},
	}

	filter := NewSceneFilter(nil, []string{typeName(Velocity{})})
	sc := NewDynamicSceneBuilder(worldReader, reg).WithFilter(filter).Build()

	if len(sc.Entities()) != 1 {
		t.Fatalf("expected 1 entity, got %d", len(sc.Entities()))
	}
	ent := sc.Entities()[0]
	for _, comp := range ent.Components {
		if comp.Reg.Name == typeName(Velocity{}) {
			t.Error("Velocity must be denied by filter")
		}
	}
}

func TestDynamicSceneBuilderExtractOnly(t *testing.T) {
	reg := buildTestRegistry()
	e1 := makeEntity(1)
	e2 := makeEntity(2)

	worldReader := &mockWorldReader{
		archs: []*mockArch{
			{
				entities: []entity.Entity{e1, e2},
				rows: []map[string]any{
					{typeName(Position{}): Position{X: 1}},
					{typeName(Position{}): Position{X: 2}},
				},
			},
		},
	}

	// Extract only e1.
	sc := NewDynamicSceneBuilder(worldReader, reg).ExtractEntity(e1.ID()).Build()
	if len(sc.Entities()) != 1 {
		t.Fatalf("ExtractEntity: expected 1 entity, got %d", len(sc.Entities()))
	}
	if sc.Entities()[0].ID != e1.ID() {
		t.Errorf("ExtractEntity: got entity %v, want %v", sc.Entities()[0].ID, e1.ID())
	}
}

// ─── SceneSpawner tests (T-3C02 + T-3C03) ───────────────────────────────────

func TestSceneSpawnerBasicSpawn(t *testing.T) {
	reg := buildTestRegistry()
	e1 := makeEntity(10)

	worldReader := &mockWorldReader{
		archs: []*mockArch{
			{
				entities: []entity.Entity{e1},
				rows: []map[string]any{
					{typeName(Position{}): Position{X: 7, Y: 8, Z: 9}},
				},
			},
		},
	}

	sc := NewDynamicSceneBuilder(worldReader, reg).Build()

	writer := newMockWorldWriter()
	spawner := NewSceneSpawner()
	id := spawner.Spawn(writer, &sc)

	if id == 0 {
		t.Fatal("Spawn must return non-zero InstanceID")
	}
	ids := spawner.InstanceEntities(id)
	if len(ids) != 1 {
		t.Fatalf("InstanceEntities: expected 1, got %d", len(ids))
	}
	comps := writer.components[ids[0]]
	if len(comps) != 1 {
		t.Fatalf("spawned entity must have 1 component, got %d", len(comps))
	}
	pos, ok := reflect.ValueOf(comps[0]).Elem().Interface().(Position)
	if !ok {
		t.Fatalf("component must be *Position, got %T", comps[0])
	}
	if pos.X != 7 || pos.Y != 8 || pos.Z != 9 {
		t.Errorf("Position mismatch: got %+v", pos)
	}
}

// TestSceneSpawnerEntityRemap verifies the two-pass entity remap (T-3C03):
// entity B references entity A via ChildOf.Parent; after spawn the reference
// must point to A's NEW entity ID, not the original pre-spawn ID.
func TestSceneSpawnerEntityRemap(t *testing.T) {
	reg := buildTestRegistry()

	eA := makeEntity(1)
	eB := makeEntity(2)

	worldReader := &mockWorldReader{
		archs: []*mockArch{
			{
				entities: []entity.Entity{eA},
				rows: []map[string]any{
					{typeName(Position{}): Position{X: 1}},
				},
			},
			{
				entities: []entity.Entity{eB},
				rows: []map[string]any{
					{typeName(ChildOf{}): ChildOf{Parent: eA.ID()}},
				},
			},
		},
	}

	sc := NewDynamicSceneBuilder(worldReader, reg).Build()
	writer := newMockWorldWriter()
	spawner := NewSceneSpawner()
	instanceID := spawner.Spawn(writer, &sc)

	allIDs := spawner.InstanceEntities(instanceID)
	if len(allIDs) != 2 {
		t.Fatalf("expected 2 spawned entities, got %d", len(allIDs))
	}

	// Find the ChildOf component among all spawned entities.
	var foundChild *ChildOf
	var newEntityAID entity.EntityID
	for _, eid := range allIDs {
		comps := writer.components[eid]
		for _, c := range comps {
			rv := reflect.ValueOf(c)
			if rv.IsNil() {
				continue
			}
			switch v := rv.Elem().Interface().(type) {
			case ChildOf:
				co := v
				foundChild = &co
			case Position:
				// This is entity A's new ID.
				newEntityAID = eid
				_ = v
			}
		}
	}

	if foundChild == nil {
		t.Fatal("ChildOf component not found in spawned entities")
	}
	if newEntityAID == 0 {
		t.Fatal("Position component (entity A) not found")
	}
	if foundChild.Parent != newEntityAID {
		t.Errorf("remap failed: ChildOf.Parent=%v, new eA ID=%v (original eA ID=%v)",
			foundChild.Parent, newEntityAID, eA.ID())
	}
	if foundChild.Parent == eA.ID() {
		t.Error("ChildOf.Parent still points to original entity ID — remap did not apply")
	}
}

func TestSceneSpawnerNilScene(t *testing.T) {
	spawner := NewSceneSpawner()
	writer := newMockWorldWriter()
	id := spawner.Spawn(writer, nil)
	if id != 0 {
		t.Errorf("Spawn(nil) must return 0, got %d", id)
	}
}

func TestSceneSpawnerEmptyScene(t *testing.T) {
	spawner := NewSceneSpawner()
	writer := newMockWorldWriter()
	empty := &DynamicScene{}
	id := spawner.Spawn(writer, empty)
	if id != 0 {
		t.Errorf("Spawn(empty) must return 0, got %d", id)
	}
}

func TestSceneSpawnerDespawn(t *testing.T) {
	reg := buildTestRegistry()
	e1 := makeEntity(1)
	worldReader := &mockWorldReader{
		archs: []*mockArch{
			{
				entities: []entity.Entity{e1},
				rows:     []map[string]any{{typeName(Position{}): Position{}}},
			},
		},
	}
	sc := NewDynamicSceneBuilder(worldReader, reg).Build()
	writer := newMockWorldWriter()
	spawner := NewSceneSpawner()
	id := spawner.Spawn(writer, &sc)

	var despawned []entity.EntityID
	spawner.DespawnInstance(id, func(eid entity.EntityID) { despawned = append(despawned, eid) })

	if spawner.InstanceEntities(id) != nil {
		t.Error("DespawnInstance must remove the instance from the spawner")
	}
	if len(despawned) != 1 {
		t.Errorf("DespawnInstance must call despawn for each entity; got %d calls", len(despawned))
	}
}

// ─── SceneFilter tests ───────────────────────────────────────────────────────

func TestSceneFilterDenyWins(t *testing.T) {
	// A type that is in both allow and deny: deny must win.
	f := NewSceneFilter([]string{"A"}, []string{"A"})
	if f.included("A") {
		t.Error("deny must win over allow")
	}
}

func TestSceneFilterAllowAll(t *testing.T) {
	f := NewSceneFilter(nil, nil)
	if !f.included("anything") {
		t.Error("empty allow-set must allow all")
	}
}

func TestSceneFilterAllowList(t *testing.T) {
	f := NewSceneFilter([]string{"X"}, nil)
	if !f.included("X") {
		t.Error("X must be included")
	}
	if f.included("Y") {
		t.Error("Y must be excluded when not in allow-list")
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// typeName returns the fully-qualified type name as the typereg would store it.
func typeName(v any) string {
	reg := typereg.NewTypeRegistry()
	t := reflect.TypeOf(v)
	reg.RegisterByType(t)
	r := reg.Resolve(t)
	if r == nil {
		return ""
	}
	return r.Name
}
