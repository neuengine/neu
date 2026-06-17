package rpc

import "encoding/binary"

// Wire format: [typeID uint16 LE][payloadLen uint16 LE][payload bytes]
const rpcHeaderSize = 4

// EncodeRpcMessage encodes typeID and payload into the RPC wire format.
// The returned slice is freshly allocated and safe to pass to a transport.
// payload length must not exceed 65535 bytes.
func EncodeRpcMessage(typeID uint16, payload []byte) []byte {
	buf := make([]byte, rpcHeaderSize+len(payload))
	binary.LittleEndian.PutUint16(buf[0:2], typeID)
	binary.LittleEndian.PutUint16(buf[2:4], uint16(len(payload)))
	copy(buf[4:], payload)
	return buf
}

// DecodeRpcMessage parses the first RPC message from data.
// Returns the type ID, the payload (aliased from data), the remaining bytes,
// and ok. Returns !ok on any truncation or malformed input without panicking.
func DecodeRpcMessage(data []byte) (typeID uint16, payload, rest []byte, ok bool) {
	if len(data) < rpcHeaderSize {
		return 0, nil, data, false
	}
	typeID = binary.LittleEndian.Uint16(data[0:2])
	plen := int(binary.LittleEndian.Uint16(data[2:4]))
	if len(data) < rpcHeaderSize+plen {
		return 0, nil, data, false
	}
	payload = data[rpcHeaderSize : rpcHeaderSize+plen]
	rest = data[rpcHeaderSize+plen:]
	return typeID, payload, rest, true
}
