package replication

import (
	"reflect"

	"github.com/neuengine/neu/internal/ecs/entity"
)

var entityIDType = reflect.TypeFor[entity.EntityID]()

// EntityMap maintains a bijective mapping between server-assigned EntityIDs
// and locally-allocated client EntityIDs. It also supports recursive remapping
// of EntityID fields within arbitrary component structs, mirroring the pattern
// used by scene.remapEntityIDs for local entity references.
type EntityMap struct {
	serverToClient map[entity.EntityID]entity.EntityID
	clientToServer map[entity.EntityID]entity.EntityID
	allocate       func() entity.EntityID
}

// NewEntityMap creates an EntityMap that calls allocate to mint a new client
// EntityID whenever a previously-unseen server EntityID arrives.
func NewEntityMap(allocate func() entity.EntityID) *EntityMap {
	return &EntityMap{
		serverToClient: make(map[entity.EntityID]entity.EntityID),
		clientToServer: make(map[entity.EntityID]entity.EntityID),
		allocate:       allocate,
	}
}

// Map returns the client EntityID for serverID, allocating one if not yet seen.
func (m *EntityMap) Map(serverID entity.EntityID) entity.EntityID {
	if clientID, ok := m.serverToClient[serverID]; ok {
		return clientID
	}
	clientID := m.allocate()
	m.serverToClient[serverID] = clientID
	m.clientToServer[clientID] = serverID
	return clientID
}

// Unmap removes the mapping for serverID and returns the former client ID.
// Returns 0 and false if serverID has no mapping.
func (m *EntityMap) Unmap(serverID entity.EntityID) (entity.EntityID, bool) {
	clientID, ok := m.serverToClient[serverID]
	if !ok {
		return 0, false
	}
	delete(m.serverToClient, serverID)
	delete(m.clientToServer, clientID)
	return clientID, true
}

// ServerOf returns the server EntityID associated with the given client ID.
func (m *EntityMap) ServerOf(clientID entity.EntityID) (entity.EntityID, bool) {
	serverID, ok := m.clientToServer[clientID]
	return serverID, ok
}

// ClientOf returns the client EntityID associated with the given server ID.
func (m *EntityMap) ClientOf(serverID entity.EntityID) (entity.EntityID, bool) {
	clientID, ok := m.serverToClient[serverID]
	return clientID, ok
}

// Len returns the number of active server→client mappings.
func (m *EntityMap) Len() int { return len(m.serverToClient) }

// Remap walks v recursively, replacing every entity.EntityID field with its
// mapped client ID (auto-allocating via Map for first-seen server IDs).
// Zero EntityID values are left untouched — they represent "no entity".
// v must be addressable so that CanSet() returns true for leaf fields.
func (m *EntityMap) Remap(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for _, field := range v.Fields() {
			m.Remap(field)
		}
	case reflect.Slice:
		for i := range v.Len() {
			m.Remap(v.Index(i))
		}
	case reflect.Pointer:
		if !v.IsNil() {
			m.Remap(v.Elem())
		}
	default:
		if v.Type() == entityIDType && v.CanSet() {
			old := entity.EntityID(v.Uint())
			if old == 0 {
				return
			}
			v.SetUint(uint64(m.Map(old)))
		}
	}
}
