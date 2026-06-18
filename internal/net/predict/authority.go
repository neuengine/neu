package predict

import netcore "github.com/neuengine/neu/internal/net"

// AuthorityMode describes how an entity's state is managed in a networked session.
type AuthorityMode uint8

const (
	// Predicted means the entity is locally simulated and rolled back on misprediction.
	// Only entities owned by the local connection carry this mode (INV-5).
	Predicted AuthorityMode = iota
	// Interpolated means the entity's display state is driven by snapshot interpolation.
	// Used for remote entities the client does not control.
	Interpolated
	// Authoritative means the entity is owned by the server and never rolled back on
	// the client — its position is authoritative and received via replication.
	Authoritative
)

// NetworkAuthority partitions entities into predicted, interpolated, and
// authoritative sets. Only entities tagged Predicted(localConn) are included in
// the prediction loop and rollback-resimulate (INV-5 query filter).
type NetworkAuthority struct {
	// Owner is the connection that controls this entity. For Predicted entities,
	// this must equal the local client's ConnectionID so systems can filter
	// their own predicted entities from remotely-controlled ones.
	Owner netcore.ConnectionID
	Mode  AuthorityMode
}
