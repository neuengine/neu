package replication

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/changedetect"
	"github.com/neuengine/neu/internal/ecs/entity"
)

func TestClientAckStateNeedsSendNoAck(t *testing.T) {
	t.Parallel()
	s := NewClientAckState()
	// No ack recorded → always need to send.
	if !s.NeedsSend(e(1), changedetect.Tick(5)) {
		t.Error("NeedsSend with no ack should return true")
	}
}

func TestClientAckStateNeedsSendAfterAck(t *testing.T) {
	t.Parallel()
	s := NewClientAckState()
	s.UpdateAck(e(1), changedetect.Tick(10))

	// Component changed at tick 9 (before ack) — already delivered.
	if s.NeedsSend(e(1), changedetect.Tick(9)) {
		t.Error("NeedsSend tick 9 ≤ ack 10 should return false")
	}
	// Component changed at tick 10 (equal to ack) — not newer.
	if s.NeedsSend(e(1), changedetect.Tick(10)) {
		t.Error("NeedsSend tick 10 == ack 10 should return false")
	}
	// Component changed at tick 11 (after ack) — needs sending.
	if !s.NeedsSend(e(1), changedetect.Tick(11)) {
		t.Error("NeedsSend tick 11 > ack 10 should return true")
	}
}

func TestClientAckStateUpdateAckAdvancesOnly(t *testing.T) {
	t.Parallel()
	s := NewClientAckState()
	s.UpdateAck(e(2), changedetect.Tick(20))
	// A lower tick must not regress the ack.
	s.UpdateAck(e(2), changedetect.Tick(5))
	// Tick 15 is newer than 5 but not 20 — must still be false.
	if s.NeedsSend(e(2), changedetect.Tick(15)) {
		t.Error("UpdateAck must not regress: ack should remain 20, not 5")
	}
}

func TestClientAckStateForget(t *testing.T) {
	t.Parallel()
	s := NewClientAckState()
	s.UpdateAck(e(3), changedetect.Tick(50))
	s.Forget(e(3))
	// After forget, any tick should need sending (fresh-spawn logic).
	if !s.NeedsSend(e(3), changedetect.Tick(1)) {
		t.Error("after Forget, NeedsSend should return true")
	}
}

func TestClientAckStateLen(t *testing.T) {
	t.Parallel()
	s := NewClientAckState()
	if s.Len() != 0 {
		t.Errorf("Len() on empty = %d, want 0", s.Len())
	}
	s.UpdateAck(entity.NewEntityID(1, 1), 1)
	s.UpdateAck(entity.NewEntityID(2, 1), 2)
	if s.Len() != 2 {
		t.Errorf("Len() = %d, want 2", s.Len())
	}
	s.Forget(entity.NewEntityID(1, 1))
	if s.Len() != 1 {
		t.Errorf("Len() after Forget = %d, want 1", s.Len())
	}
}

func TestClientAckStateMultipleEntities(t *testing.T) {
	t.Parallel()
	s := NewClientAckState()
	s.UpdateAck(e(1), changedetect.Tick(5))
	s.UpdateAck(e(2), changedetect.Tick(10))

	// e(1): ack=5; tick 6 is newer.
	if !s.NeedsSend(e(1), changedetect.Tick(6)) {
		t.Error("e(1): tick 6 > ack 5 should need sending")
	}
	// e(2): ack=10; tick 6 is older.
	if s.NeedsSend(e(2), changedetect.Tick(6)) {
		t.Error("e(2): tick 6 ≤ ack 10 should not need sending")
	}
}
