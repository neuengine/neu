package replication

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/entity"
)

func TestEncodeDecodeEntitySpawn(t *testing.T) {
	t.Parallel()
	msg := ReplicationMessage{
		Kind:     MsgEntitySpawn,
		ServerID: entity.NewEntityID(5, 1),
		Components: []ReplicatedComponent{
			{TypeName: "pkg.Position", Data: []byte(`{"X":1,"Y":2}`)},
			{TypeName: "pkg.Velocity", Data: []byte(`{"Dx":0.5}`)},
		},
	}
	data := EncodeReplicationMessage(msg)
	got, rest, ok := DecodeReplicationMessage(data)
	if !ok {
		t.Fatal("DecodeReplicationMessage !ok")
	}
	if len(rest) != 0 {
		t.Errorf("rest not empty: %d bytes", len(rest))
	}
	if got.Kind != MsgEntitySpawn {
		t.Errorf("Kind = %v, want MsgEntitySpawn", got.Kind)
	}
	if got.ServerID != msg.ServerID {
		t.Errorf("ServerID = %v, want %v", got.ServerID, msg.ServerID)
	}
	if len(got.Components) != 2 {
		t.Fatalf("Components = %d, want 2", len(got.Components))
	}
	if got.Components[0].TypeName != "pkg.Position" {
		t.Errorf("TypeName[0] = %q", got.Components[0].TypeName)
	}
	if string(got.Components[0].Data) != `{"X":1,"Y":2}` {
		t.Errorf("Data[0] = %q", got.Components[0].Data)
	}
	if got.Components[1].TypeName != "pkg.Velocity" {
		t.Errorf("TypeName[1] = %q", got.Components[1].TypeName)
	}
}

func TestEncodeDecodeEntityDespawn(t *testing.T) {
	t.Parallel()
	msg := ReplicationMessage{Kind: MsgEntityDespawn, ServerID: entity.NewEntityID(3, 2)}
	data := EncodeReplicationMessage(msg)
	got, rest, ok := DecodeReplicationMessage(data)
	if !ok {
		t.Fatal("DecodeReplicationMessage !ok")
	}
	if len(rest) != 0 {
		t.Errorf("rest not empty")
	}
	if got.Kind != MsgEntityDespawn || got.ServerID != msg.ServerID {
		t.Errorf("got = %+v", got)
	}
}

func TestEncodeDecodeComponentRemove(t *testing.T) {
	t.Parallel()
	msg := ReplicationMessage{
		Kind:      MsgComponentRemove,
		ServerID:  entity.NewEntityID(7, 1),
		TypeNames: []string{"pkg.Health", "pkg.Shield"},
	}
	data := EncodeReplicationMessage(msg)
	got, _, ok := DecodeReplicationMessage(data)
	if !ok {
		t.Fatal("DecodeReplicationMessage !ok")
	}
	if got.Kind != MsgComponentRemove {
		t.Errorf("Kind = %v", got.Kind)
	}
	if len(got.TypeNames) != 2 || got.TypeNames[0] != "pkg.Health" || got.TypeNames[1] != "pkg.Shield" {
		t.Errorf("TypeNames = %v", got.TypeNames)
	}
}

func TestEncodeDecodeComponentUpdate(t *testing.T) {
	t.Parallel()
	msg := ReplicationMessage{
		Kind:     MsgComponentUpdate,
		ServerID: entity.NewEntityID(1, 1),
		Components: []ReplicatedComponent{
			{TypeName: "pkg.HP", Data: []byte(`{"Val":100}`)},
		},
	}
	data := EncodeReplicationMessage(msg)
	got, _, ok := DecodeReplicationMessage(data)
	if !ok {
		t.Fatal("!ok")
	}
	if got.Kind != MsgComponentUpdate {
		t.Errorf("Kind = %v", got.Kind)
	}
}

func TestDecodeReplicationMessageTruncated(t *testing.T) {
	t.Parallel()
	valid := EncodeReplicationMessage(ReplicationMessage{
		Kind:     MsgEntitySpawn,
		ServerID: entity.NewEntityID(1, 1),
		Components: []ReplicatedComponent{
			{TypeName: "A", Data: []byte("x")},
		},
	})
	// Truncate at every possible length — none should panic or return ok.
	for cut := 0; cut < len(valid)-1; cut++ {
		_, _, ok := DecodeReplicationMessage(valid[:cut])
		if ok {
			t.Errorf("DecodeReplicationMessage([:%d]) = ok on truncated data", cut)
		}
	}
}

func TestDecodeReplicationMessageChained(t *testing.T) {
	t.Parallel()
	m1 := EncodeReplicationMessage(ReplicationMessage{Kind: MsgEntityDespawn, ServerID: entity.NewEntityID(1, 1)})
	m2 := EncodeReplicationMessage(ReplicationMessage{Kind: MsgEntityDespawn, ServerID: entity.NewEntityID(2, 1)})
	data := append(m1, m2...)

	g1, rest, ok := DecodeReplicationMessage(data)
	if !ok || g1.ServerID != entity.NewEntityID(1, 1) {
		t.Fatalf("first decode: ok=%v id=%v", ok, g1.ServerID)
	}
	g2, rest, ok := DecodeReplicationMessage(rest)
	if !ok || g2.ServerID != entity.NewEntityID(2, 1) {
		t.Fatalf("second decode: ok=%v id=%v", ok, g2.ServerID)
	}
	if len(rest) != 0 {
		t.Errorf("rest not empty after two messages: %d bytes", len(rest))
	}
}

func FuzzDecodeReplicationMessage(f *testing.F) {
	spawn := EncodeReplicationMessage(ReplicationMessage{
		Kind:       MsgEntitySpawn,
		ServerID:   entity.NewEntityID(1, 1),
		Components: []ReplicatedComponent{{TypeName: "T", Data: []byte(`{}`)}},
	})
	f.Add(spawn)
	f.Add(EncodeReplicationMessage(ReplicationMessage{Kind: MsgEntityDespawn, ServerID: entity.NewEntityID(2, 1)}))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		msg, _, ok := DecodeReplicationMessage(data)
		if !ok {
			return
		}
		// Re-encode must decode identically.
		enc := EncodeReplicationMessage(msg)
		msg2, rest2, ok2 := DecodeReplicationMessage(enc)
		if !ok2 {
			t.Fatal("round-trip: encoded message could not be decoded")
		}
		if msg2.Kind != msg.Kind || msg2.ServerID != msg.ServerID {
			t.Fatalf("round-trip mismatch: kind %v→%v serverID %v→%v", msg.Kind, msg2.Kind, msg.ServerID, msg2.ServerID)
		}
		if len(rest2) != 0 {
			t.Fatalf("round-trip: rest not empty (%d bytes)", len(rest2))
		}
	})
}
