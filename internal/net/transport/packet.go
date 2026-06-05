// Package transport implements the UDP realization of the
// github.com/neuengine/neu/internal/net NetworkTransport abstraction: a
// wire-format codec, a user-space reliability layer (sliding window, piggybacked
// ACKs, RTO retransmit, reorder + duplicate detection), connection lifecycle,
// and Path-MTU discovery. It uses only the standard library (encoding/binary,
// net, time, hash) per the engine's zero-dependency rule. [Bootstrap]
package transport

import (
	"encoding/binary"
	"errors"
)

// ProtocolID is the magic value in every datagram header; a datagram whose
// header carries a different value is from a foreign sender and is rejected.
const ProtocolID uint16 = 0x4E55 // "NU"

// MaxDatagramSize is the conservative MTU cap (bytes) before fragmentation.
const MaxDatagramSize = 1200

// Datagram header flags.
const (
	// FlagAck marks that an ACK block follows the header.
	FlagAck uint8 = 1 << iota
	// FlagFragment marks a fragment of a larger message.
	FlagFragment
	// FlagCompressed marks a compressed payload (reserved).
	FlagCompressed
)

const (
	headerSize = 8 // protocolID(2) connID(2) packetSeq(2) flags(1) channelCount(1)
	ackSize    = 6 // ackBase(2) ackBits(4)
	frameHead  = 5 // channelID(1) msgSeq(2) payloadLen(2)
)

// ErrMalformedPacket is returned by Decode for a corrupt or truncated datagram.
var ErrMalformedPacket = errors.New("transport: malformed packet")

// Header is a decoded datagram header.
type Header struct {
	ConnectionID uint16
	PacketSeq    uint16
	Flags        uint8
	ChannelCount uint8
}

// Frame is one channel message inside a datagram.
type Frame struct {
	Payload []byte
	MsgSeq  uint16
	Channel uint8
}

// Ack is the piggybacked acknowledgement block: AckBase is the highest received
// packet sequence, AckBits a bitfield of the 32 preceding sequences (bit i set =
// AckBase-(i+1) received).
type Ack struct {
	Base uint16
	Bits uint32
}

// Encode serializes a datagram. When ack is non-nil, FlagAck is set and the ACK
// block is written after the header. Frames are length-prefixed. The caller is
// responsible for keeping the total under MaxDatagramSize (Encode does not
// fragment — fragmentation is a higher layer).
func Encode(connID, packetSeq uint16, flags uint8, ack *Ack, frames []Frame) []byte {
	size := headerSize
	if ack != nil {
		flags |= FlagAck
		size += ackSize
	}
	for _, f := range frames {
		size += frameHead + len(f.Payload)
	}
	buf := make([]byte, size)
	binary.LittleEndian.PutUint16(buf[0:], ProtocolID)
	binary.LittleEndian.PutUint16(buf[2:], connID)
	binary.LittleEndian.PutUint16(buf[4:], packetSeq)
	buf[6] = flags
	buf[7] = uint8(len(frames))
	off := headerSize
	if ack != nil {
		binary.LittleEndian.PutUint16(buf[off:], ack.Base)
		binary.LittleEndian.PutUint32(buf[off+2:], ack.Bits)
		off += ackSize
	}
	for _, f := range frames {
		buf[off] = f.Channel
		binary.LittleEndian.PutUint16(buf[off+1:], f.MsgSeq)
		binary.LittleEndian.PutUint16(buf[off+3:], uint16(len(f.Payload)))
		off += frameHead
		off += copy(buf[off:], f.Payload)
	}
	return buf
}

// Decode parses a datagram. It validates the protocol ID and bounds-checks every
// length field against the remaining buffer before slicing, so a truncated or
// hostile datagram returns ErrMalformedPacket rather than panicking. The
// returned frame payloads alias data — copy if retained.
func Decode(data []byte) (Header, *Ack, []Frame, error) {
	if len(data) < headerSize {
		return Header{}, nil, nil, ErrMalformedPacket
	}
	if binary.LittleEndian.Uint16(data[0:]) != ProtocolID {
		return Header{}, nil, nil, ErrMalformedPacket
	}
	h := Header{
		ConnectionID: binary.LittleEndian.Uint16(data[2:]),
		PacketSeq:    binary.LittleEndian.Uint16(data[4:]),
		Flags:        data[6],
		ChannelCount: data[7],
	}
	off := headerSize

	var ack *Ack
	if h.Flags&FlagAck != 0 {
		if len(data)-off < ackSize {
			return Header{}, nil, nil, ErrMalformedPacket
		}
		ack = &Ack{
			Base: binary.LittleEndian.Uint16(data[off:]),
			Bits: binary.LittleEndian.Uint32(data[off+2:]),
		}
		off += ackSize
	}

	frames := make([]Frame, 0, h.ChannelCount)
	for range h.ChannelCount {
		if len(data)-off < frameHead {
			return Header{}, nil, nil, ErrMalformedPacket
		}
		ch := data[off]
		seq := binary.LittleEndian.Uint16(data[off+1:])
		plen := int(binary.LittleEndian.Uint16(data[off+3:]))
		off += frameHead
		if len(data)-off < plen {
			return Header{}, nil, nil, ErrMalformedPacket
		}
		frames = append(frames, Frame{Channel: ch, MsgSeq: seq, Payload: data[off : off+plen]})
		off += plen
	}
	return h, ack, frames, nil
}
