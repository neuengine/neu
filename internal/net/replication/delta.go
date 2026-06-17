package replication

import (
	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/entity"
)

// ClientAckState tracks the last-acknowledged change tick per entity for one
// client connection. It enables delta selection: only components changed after
// the acknowledged tick are included in the next replication message.
//
// Ticks are sourced from the change-detection system (changedetect.Tick),
// which already tracks mutations per-component — no parallel dirty state.
type ClientAckState struct {
	lastAck map[entity.EntityID]changedetect.Tick
}

// NewClientAckState returns an initialized ClientAckState.
func NewClientAckState() *ClientAckState {
	return &ClientAckState{lastAck: make(map[entity.EntityID]changedetect.Tick)}
}

// UpdateAck records that the client has acknowledged component state for e
// up to tick. Subsequent NeedsSend calls will only return true for components
// changed strictly after tick.
func (s *ClientAckState) UpdateAck(e entity.EntityID, tick changedetect.Tick) {
	current, ok := s.lastAck[e]
	if !ok || tick > current {
		s.lastAck[e] = tick
	}
}

// NeedsSend reports whether a component at compTick must be sent to the client.
// Returns true when compTick is newer than the last-acked tick for e
// (i.e., the client has not yet received this version).
func (s *ClientAckState) NeedsSend(e entity.EntityID, compTick changedetect.Tick) bool {
	last, ok := s.lastAck[e]
	if !ok {
		return true // no ack yet → always send
	}
	return compTick.IsNewerThan(last)
}

// Forget removes the ack record for e. Call on visibility-leave or despawn
// so that if e later re-enters visibility, it gets a full spawn snapshot.
func (s *ClientAckState) Forget(e entity.EntityID) {
	delete(s.lastAck, e)
}

// Len returns the number of entity records in the ack state.
func (s *ClientAckState) Len() int { return len(s.lastAck) }

// DeltaSerializer provides optional byte-level diff/patch for a component type.
// When registered for a type, the send system calls Diff to produce a compact
// patch; the receive system calls Patch to reconstruct the full component.
// Absent registration → full component data is sent every time.
type DeltaSerializer interface {
	// Diff computes a patch from old to new. Returns changed=false when
	// the component is unchanged (identical bytes), in which case no message
	// is emitted even if the change tick is newer.
	Diff(old, newData []byte) (patch []byte, changed bool)
	// Patch applies patch on top of base and returns the reconstructed value.
	Patch(base, patch []byte) []byte
}
