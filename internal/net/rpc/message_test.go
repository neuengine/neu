package rpc

import "testing"

func TestEncodeDecodeRpcMessageRoundTrip(t *testing.T) {
	t.Parallel()
	payload := []byte("hello world")
	msg := EncodeRpcMessage(42, payload)

	typeID, got, rest, ok := DecodeRpcMessage(msg)
	if !ok {
		t.Fatal("DecodeRpcMessage returned !ok")
	}
	if typeID != 42 {
		t.Errorf("typeID = %d, want 42", typeID)
	}
	if string(got) != "hello world" {
		t.Errorf("payload = %q, want %q", got, payload)
	}
	if len(rest) != 0 {
		t.Errorf("rest not empty after single-message decode: %v", rest)
	}
}

func TestDecodeRpcMessageEmptyPayload(t *testing.T) {
	t.Parallel()
	msg := EncodeRpcMessage(7, nil)
	typeID, payload, rest, ok := DecodeRpcMessage(msg)
	if !ok {
		t.Fatal("DecodeRpcMessage(!ok) for empty payload")
	}
	if typeID != 7 {
		t.Errorf("typeID = %d, want 7", typeID)
	}
	if len(payload) != 0 {
		t.Errorf("payload len = %d, want 0", len(payload))
	}
	if len(rest) != 0 {
		t.Errorf("rest not empty: %v", rest)
	}
}

func TestDecodeRpcMessageTruncated(t *testing.T) {
	t.Parallel()
	cases := [][]byte{
		{},
		{1},
		{1, 0},
		{1, 0, 5},                   // header cut short
		{1, 0, 5, 0, 1, 2},          // payloadLen=5, only 2 bytes follow
	}
	for _, c := range cases {
		_, _, _, ok := DecodeRpcMessage(c)
		if ok {
			t.Errorf("DecodeRpcMessage(%v) = ok, want false (truncated)", c)
		}
	}
}

func TestDecodeRpcMessageChained(t *testing.T) {
	t.Parallel()
	msg1 := EncodeRpcMessage(1, []byte("a"))
	msg2 := EncodeRpcMessage(2, []byte("bb"))
	data := append(msg1, msg2...)

	id, p, rest, ok := DecodeRpcMessage(data)
	if !ok || id != 1 || string(p) != "a" {
		t.Errorf("first decode: id=%d p=%q ok=%v", id, p, ok)
	}
	id, p, rest, ok = DecodeRpcMessage(rest)
	if !ok || id != 2 || string(p) != "bb" {
		t.Errorf("second decode: id=%d p=%q ok=%v rest=%v", id, p, ok, rest)
	}
	if len(rest) != 0 {
		t.Errorf("rest not empty after two decodes: %v", rest)
	}
}

func FuzzDecodeRpcMessage(f *testing.F) {
	f.Add([]byte{1, 0, 3, 0, 'a', 'b', 'c'})
	f.Add([]byte{0xff, 0xff, 0x00, 0x00})
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		typeID, payload, rest, ok := DecodeRpcMessage(data)
		if !ok {
			return
		}
		// Round-trip invariant: what we decoded must re-encode cleanly.
		roundtrip := EncodeRpcMessage(typeID, payload)
		id2, p2, r2, ok2 := DecodeRpcMessage(roundtrip)
		if !ok2 {
			t.Fatal("round-trip: encoded message could not be decoded")
		}
		if id2 != typeID || string(p2) != string(payload) || len(r2) != 0 {
			t.Fatalf("round-trip mismatch: typeID=%d→%d payload=%q→%q rest=%v",
				typeID, id2, payload, p2, r2)
		}
		_ = rest
	})
}
