package replication

import (
	"encoding/binary"

	"github.com/neuengine/neu/internal/ecs/entity"
)

// MsgKind identifies the type of a replication message.
type MsgKind uint8

const (
	// MsgEntitySpawn carries a full component snapshot for a newly-visible entity.
	MsgEntitySpawn MsgKind = 1
	// MsgComponentUpdate carries changed component data for an existing entity.
	MsgComponentUpdate MsgKind = 2
	// MsgComponentRemove signals that one or more components were removed.
	MsgComponentRemove MsgKind = 3
	// MsgEntityDespawn signals that an entity left visibility or was destroyed.
	MsgEntityDespawn MsgKind = 4
)

// ReplicatedComponent is one serialized component in a replication message.
// TypeName is the fully-qualified Go type name used by the type registry.
// Data is the JSON-encoded component value.
type ReplicatedComponent struct {
	TypeName string
	Data     []byte
}

// ReplicationMessage is one replication wire message.
type ReplicationMessage struct {
	Kind       MsgKind
	ServerID   entity.EntityID
	Components []ReplicatedComponent // non-nil for Spawn/Update
	TypeNames  []string              // non-nil for ComponentRemove
}

// Wire format:
//   [Kind uint8][ServerID uint64 LE]
//   EntitySpawn / ComponentUpdate:
//     [NumComponents uint16 LE]
//     For each: [TypeNameLen uint16 LE][TypeName bytes][DataLen uint32 LE][Data bytes]
//   ComponentRemove:
//     [NumTypeNames uint16 LE]
//     For each: [TypeNameLen uint16 LE][TypeName bytes]
//   EntityDespawn: (nothing)

const replMsgMinSize = 9 // Kind(1) + ServerID(8)

// EncodeReplicationMessage serializes msg into a newly-allocated byte slice.
func EncodeReplicationMessage(msg ReplicationMessage) []byte {
	switch msg.Kind {
	case MsgEntitySpawn, MsgComponentUpdate:
		return encodeWithComponents(msg)
	case MsgComponentRemove:
		return encodeComponentRemove(msg)
	default: // MsgEntityDespawn
		buf := make([]byte, replMsgMinSize)
		buf[0] = byte(msg.Kind)
		binary.LittleEndian.PutUint64(buf[1:9], uint64(msg.ServerID))
		return buf
	}
}

func encodeWithComponents(msg ReplicationMessage) []byte {
	// Calculate total size first.
	size := replMsgMinSize + 2 // Kind + ServerID + NumComponents
	for _, c := range msg.Components {
		size += 2 + len(c.TypeName) + 4 + len(c.Data)
	}
	buf := make([]byte, size)
	off := 0
	buf[off] = byte(msg.Kind)
	off++
	binary.LittleEndian.PutUint64(buf[off:], uint64(msg.ServerID))
	off += 8
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(msg.Components)))
	off += 2
	for _, c := range msg.Components {
		binary.LittleEndian.PutUint16(buf[off:], uint16(len(c.TypeName)))
		off += 2
		copy(buf[off:], c.TypeName)
		off += len(c.TypeName)
		binary.LittleEndian.PutUint32(buf[off:], uint32(len(c.Data)))
		off += 4
		copy(buf[off:], c.Data)
		off += len(c.Data)
	}
	return buf
}

func encodeComponentRemove(msg ReplicationMessage) []byte {
	size := replMsgMinSize + 2
	for _, n := range msg.TypeNames {
		size += 2 + len(n)
	}
	buf := make([]byte, size)
	off := 0
	buf[off] = byte(msg.Kind)
	off++
	binary.LittleEndian.PutUint64(buf[off:], uint64(msg.ServerID))
	off += 8
	binary.LittleEndian.PutUint16(buf[off:], uint16(len(msg.TypeNames)))
	off += 2
	for _, n := range msg.TypeNames {
		binary.LittleEndian.PutUint16(buf[off:], uint16(len(n)))
		off += 2
		copy(buf[off:], n)
		off += len(n)
	}
	return buf
}

// DecodeReplicationMessage parses one message from data.
// Returns the message, remaining bytes, and ok. !ok on any truncation.
func DecodeReplicationMessage(data []byte) (msg ReplicationMessage, rest []byte, ok bool) {
	if len(data) < replMsgMinSize {
		return ReplicationMessage{}, data, false
	}
	msg.Kind = MsgKind(data[0])
	msg.ServerID = entity.EntityID(binary.LittleEndian.Uint64(data[1:9]))
	data = data[replMsgMinSize:]

	switch msg.Kind {
	case MsgEntitySpawn, MsgComponentUpdate:
		if len(data) < 2 {
			return ReplicationMessage{}, nil, false
		}
		numComp := int(binary.LittleEndian.Uint16(data[0:2]))
		data = data[2:]
		msg.Components = make([]ReplicatedComponent, 0, numComp)
		for range numComp {
			if len(data) < 2 {
				return ReplicationMessage{}, nil, false
			}
			tnLen := int(binary.LittleEndian.Uint16(data[0:2]))
			data = data[2:]
			if len(data) < tnLen {
				return ReplicationMessage{}, nil, false
			}
			typeName := string(data[:tnLen])
			data = data[tnLen:]
			if len(data) < 4 {
				return ReplicationMessage{}, nil, false
			}
			dLen := int(binary.LittleEndian.Uint32(data[0:4]))
			data = data[4:]
			if len(data) < dLen {
				return ReplicationMessage{}, nil, false
			}
			compData := make([]byte, dLen)
			copy(compData, data[:dLen])
			data = data[dLen:]
			msg.Components = append(msg.Components, ReplicatedComponent{TypeName: typeName, Data: compData})
		}

	case MsgComponentRemove:
		if len(data) < 2 {
			return ReplicationMessage{}, nil, false
		}
		numNames := int(binary.LittleEndian.Uint16(data[0:2]))
		data = data[2:]
		msg.TypeNames = make([]string, 0, numNames)
		for range numNames {
			if len(data) < 2 {
				return ReplicationMessage{}, nil, false
			}
			tnLen := int(binary.LittleEndian.Uint16(data[0:2]))
			data = data[2:]
			if len(data) < tnLen {
				return ReplicationMessage{}, nil, false
			}
			msg.TypeNames = append(msg.TypeNames, string(data[:tnLen]))
			data = data[tnLen:]
		}

	default: // MsgEntityDespawn and unknown — no extra bytes
	}
	return msg, data, true
}
