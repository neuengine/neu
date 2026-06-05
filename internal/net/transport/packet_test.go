package transport

import (
	"bytes"
	"testing"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	frames := []Frame{
		{Channel: 0, MsgSeq: 10, Payload: []byte("pos")},
		{Channel: 1, MsgSeq: 20, Payload: []byte("event-data")},
	}
	ack := &Ack{Base: 99, Bits: 0b1011}
	data := Encode(42, 7, 0, ack, frames)

	h, gotAck, gotFrames, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if h.ConnectionID != 42 || h.PacketSeq != 7 || h.ChannelCount != 2 {
		t.Errorf("header = %+v", h)
	}
	if h.Flags&FlagAck == 0 {
		t.Error("FlagAck should be set when an ack is encoded")
	}
	if gotAck == nil || gotAck.Base != 99 || gotAck.Bits != 0b1011 {
		t.Errorf("ack = %+v", gotAck)
	}
	if len(gotFrames) != 2 {
		t.Fatalf("got %d frames, want 2", len(gotFrames))
	}
	if gotFrames[0].Channel != 0 || gotFrames[0].MsgSeq != 10 || !bytes.Equal(gotFrames[0].Payload, []byte("pos")) {
		t.Errorf("frame 0 = %+v", gotFrames[0])
	}
	if gotFrames[1].Channel != 1 || gotFrames[1].MsgSeq != 20 || string(gotFrames[1].Payload) != "event-data" {
		t.Errorf("frame 1 = %+v", gotFrames[1])
	}
}

func TestEncodeDecodeNoAck(t *testing.T) {
	t.Parallel()
	data := Encode(1, 1, 0, nil, []Frame{{Channel: 2, MsgSeq: 5, Payload: []byte("x")}})
	h, ack, frames, err := Decode(data)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if h.Flags&FlagAck != 0 || ack != nil {
		t.Error("no ack expected")
	}
	if len(frames) != 1 || frames[0].Channel != 2 {
		t.Errorf("frames = %+v", frames)
	}
}

func TestDecodeRejectsForeignAndTruncated(t *testing.T) {
	t.Parallel()
	// Too short for a header.
	if _, _, _, err := Decode([]byte{1, 2, 3}); err == nil {
		t.Error("short header should error")
	}
	// Wrong protocol id.
	bad := Encode(1, 1, 0, nil, nil)
	bad[0], bad[1] = 0xFF, 0xFF
	if _, _, _, err := Decode(bad); err == nil {
		t.Error("foreign protocol id should error")
	}
	// Truncated payload: claim a frame with a payload longer than present.
	good := Encode(1, 1, 0, nil, []Frame{{Channel: 0, MsgSeq: 1, Payload: []byte("hello")}})
	if _, _, _, err := Decode(good[:len(good)-3]); err == nil {
		t.Error("truncated payload should error")
	}
	// FlagAck set but no ack block.
	hdr := Encode(1, 1, 0, nil, nil)
	hdr[6] |= FlagAck
	if _, _, _, err := Decode(hdr); err == nil {
		t.Error("FlagAck without ack block should error")
	}
}

// FuzzDecode asserts the decoder never panics on arbitrary input (UDP carries
// untrusted bytes) — CLAUDE.md fuzz mandate.
func FuzzDecode(f *testing.F) {
	f.Add(Encode(1, 1, 0, &Ack{Base: 5, Bits: 7}, []Frame{{Channel: 0, MsgSeq: 1, Payload: []byte("a")}}))
	f.Add([]byte{0x55, 0x4E, 0, 0, 0, 0, 0xFF, 0xFF}) // header claiming 255 frames
	f.Add([]byte("not a packet"))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _, _, _ = Decode(data) // must not panic
	})
}
