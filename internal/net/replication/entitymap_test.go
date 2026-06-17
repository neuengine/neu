package replication

import (
	"reflect"
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
)

func makeAllocator() func() entity.EntityID {
	var n uint32
	return func() entity.EntityID {
		n++
		return entity.NewEntityID(n, 1)
	}
}

func TestEntityMapMapBijective(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(10, 1)

	clientID1 := em.Map(serverID)
	if clientID1 == 0 {
		t.Fatal("Map returned zero EntityID")
	}
	clientID2 := em.Map(serverID)
	if clientID1 != clientID2 {
		t.Errorf("Map called twice: got %v then %v, want same ID", clientID1, clientID2)
	}
	if em.Len() != 1 {
		t.Errorf("Len() = %d, want 1 after one unique server ID", em.Len())
	}
}

func TestEntityMapMapTwoIDs(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	s1 := entity.NewEntityID(1, 1)
	s2 := entity.NewEntityID(2, 1)
	c1 := em.Map(s1)
	c2 := em.Map(s2)
	if c1 == c2 {
		t.Errorf("distinct server IDs mapped to same client ID: %v", c1)
	}
	if em.Len() != 2 {
		t.Errorf("Len() = %d, want 2", em.Len())
	}
}

func TestEntityMapUnmap(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(7, 2)
	clientID := em.Map(serverID)

	got, ok := em.Unmap(serverID)
	if !ok || got != clientID {
		t.Errorf("Unmap() = %v, %v; want %v, true", got, ok, clientID)
	}
	if em.Len() != 0 {
		t.Errorf("Len() after Unmap = %d, want 0", em.Len())
	}
	if _, ok = em.ServerOf(clientID); ok {
		t.Error("ServerOf(clientID) should return false after Unmap")
	}
	if _, ok = em.Unmap(serverID); ok {
		t.Error("second Unmap should return false")
	}
}

func TestEntityMapServerOf(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(3, 1)
	clientID := em.Map(serverID)

	got, ok := em.ServerOf(clientID)
	if !ok || got != serverID {
		t.Errorf("ServerOf(%v) = %v, %v; want %v, true", clientID, got, ok, serverID)
	}
	_, ok = em.ServerOf(entity.NewEntityID(999, 9))
	if ok {
		t.Error("ServerOf for unknown client ID should return false")
	}
}

func TestEntityMapClientOf(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(5, 1)
	clientID := em.Map(serverID)

	got, ok := em.ClientOf(serverID)
	if !ok || got != clientID {
		t.Errorf("ClientOf(%v) = %v, %v; want %v, true", serverID, got, ok, clientID)
	}
	_, ok = em.ClientOf(entity.NewEntityID(999, 9))
	if ok {
		t.Error("ClientOf for unknown server ID should return false")
	}
}

func TestEntityMapRemapSubstitutesField(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(100, 1)
	expectedClientID := em.Map(serverID)

	type Pos struct {
		Owner entity.EntityID
		X, Y  float32
	}
	p := Pos{Owner: serverID, X: 1.5, Y: 2.5}
	em.Remap(reflect.ValueOf(&p).Elem())

	if p.Owner != expectedClientID {
		t.Errorf("Remap: Owner = %v, want %v", p.Owner, expectedClientID)
	}
	if p.X != 1.5 || p.Y != 2.5 {
		t.Errorf("Remap modified non-EntityID fields: %+v", p)
	}
}

func TestEntityMapRemapAllocatesNew(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(77, 3)

	type Link struct{ Target entity.EntityID }
	link := Link{Target: serverID}
	em.Remap(reflect.ValueOf(&link).Elem())

	if em.Len() != 1 {
		t.Errorf("Remap should auto-allocate: Len() = %d, want 1", em.Len())
	}
	clientID, ok := em.ClientOf(serverID)
	if !ok || clientID == 0 {
		t.Errorf("ClientOf after Remap: %v, %v", clientID, ok)
	}
	if link.Target != clientID {
		t.Errorf("link.Target = %v, want %v", link.Target, clientID)
	}
}

func TestEntityMapRemapZeroUntouched(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())

	type Data struct{ Ref entity.EntityID }
	d := Data{Ref: 0}
	em.Remap(reflect.ValueOf(&d).Elem())

	if d.Ref != 0 {
		t.Errorf("Remap changed zero EntityID: got %v", d.Ref)
	}
	if em.Len() != 0 {
		t.Errorf("Remap allocated for zero EntityID: Len() = %d", em.Len())
	}
}

func TestEntityMapRemapSlice(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	s1 := entity.NewEntityID(1, 1)
	s2 := entity.NewEntityID(2, 1)
	c1 := em.Map(s1)
	c2 := em.Map(s2)

	type Team struct{ Members []entity.EntityID }
	team := Team{Members: []entity.EntityID{s1, s2}}
	em.Remap(reflect.ValueOf(&team).Elem())

	if team.Members[0] != c1 || team.Members[1] != c2 {
		t.Errorf("Remap slice: got %v, want [%v %v]", team.Members, c1, c2)
	}
}

func TestEntityMapRemapNestedStruct(t *testing.T) {
	t.Parallel()
	em := NewEntityMap(makeAllocator())
	serverID := entity.NewEntityID(42, 1)
	clientID := em.Map(serverID)

	type Inner struct{ ID entity.EntityID }
	type Outer struct {
		A Inner
		B Inner
	}
	o := Outer{A: Inner{ID: serverID}, B: Inner{ID: 0}}
	em.Remap(reflect.ValueOf(&o).Elem())

	if o.A.ID != clientID {
		t.Errorf("nested A.ID = %v, want %v", o.A.ID, clientID)
	}
	if o.B.ID != 0 {
		t.Errorf("nested B.ID should remain 0, got %v", o.B.ID)
	}
}
