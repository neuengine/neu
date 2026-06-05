package transport

import (
	"testing"
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

func TestSeqGreaterWraparound(t *testing.T) {
	t.Parallel()
	if !seqGreater(5, 3) || seqGreater(3, 5) || seqGreater(5, 5) {
		t.Error("basic seqGreater wrong")
	}
	// Wraparound: 1 is "after" 65535.
	if !seqGreater(1, 65535) || seqGreater(65535, 1) {
		t.Error("wraparound seqGreater wrong")
	}
}

func TestRTOEstimator(t *testing.T) {
	t.Parallel()
	var e rtoEstimator
	if e.rto() != minRTO {
		t.Errorf("initial rto = %v, want minRTO", e.rto())
	}
	e.sample(100 * time.Millisecond)
	// After one sample SRTT=100ms, RTTVAR=50ms → RTO=300ms, clamped within range.
	if got := e.rto(); got < minRTO || got > maxRTO {
		t.Errorf("rto out of range: %v", got)
	}
	// A huge sample is clamped to maxRTO.
	e.sample(10 * time.Second)
	if e.rto() != maxRTO {
		t.Errorf("rto = %v, want clamped to maxRTO", e.rto())
	}
}

func TestAckTrackerBitfield(t *testing.T) {
	t.Parallel()
	var tr ackTracker
	if tr.ack() != nil {
		t.Error("empty tracker should produce no ack")
	}
	tr.onReceived(10)
	tr.onReceived(11)
	tr.onReceived(13) // 12 missing
	a := tr.ack()
	if a == nil || a.Base != 13 {
		t.Fatalf("ack base = %+v, want 13", a)
	}
	if !acked(*a, 13) || !acked(*a, 11) || !acked(*a, 10) {
		t.Error("received seqs should be acked")
	}
	if acked(*a, 12) {
		t.Error("missing seq 12 must not be acked")
	}
	// Out-of-order older arrival fills a gap.
	tr.onReceived(12)
	if !acked(*tr.ack(), 12) {
		t.Error("late 12 should now be acked")
	}
}

func TestReliableSenderWindowAndRTT(t *testing.T) {
	t.Parallel()
	s := newReliableSender(4)
	t0 := time.Unix(0, 0)
	seq, ok := s.queue([]byte("a"))
	if !ok || seq != 0 || s.pending() != 1 {
		t.Fatalf("queue: seq=%d ok=%v pending=%d", seq, ok, s.pending())
	}
	s.queue([]byte("b"))

	// First due() emits both never-sent frames.
	due := s.due(t0)
	if len(due) != 2 {
		t.Fatalf("first due = %d frames, want 2", len(due))
	}
	// Immediately after, nothing is due (RTO not elapsed).
	if len(s.due(t0.Add(time.Millisecond))) != 0 {
		t.Error("nothing should be due before RTO")
	}
	// After RTO, frame 0 is retransmitted.
	later := t0.Add(maxRTO + time.Second)
	if len(s.due(later)) != 2 {
		t.Error("frames should be retransmitted after RTO")
	}

	// ACK frame 0 → drops from window. (Retransmitted → no RTT sample.)
	s.onAck(Ack{Base: 0}, later.Add(50*time.Millisecond))
	if s.pending() != 1 {
		t.Errorf("after acking seq 0, pending = %d, want 1", s.pending())
	}

	// A fresh sender: ack-on-first-transmit samples RTT.
	s2 := newReliableSender(4)
	s2.queue([]byte("x"))
	s2.due(t0) // first send at t0
	s2.onAck(Ack{Base: 0}, t0.Add(80*time.Millisecond))
	if !s2.est.have {
		t.Error("RTT should be sampled when acked on first transmission")
	}
}

func TestReliableSenderWindowFull(t *testing.T) {
	t.Parallel()
	s := newReliableSender(2)
	s.queue([]byte("a"))
	s.queue([]byte("b"))
	if _, ok := s.queue([]byte("c")); ok {
		t.Error("queue past window cap should fail")
	}
}

func TestReliableReceiverDedup(t *testing.T) {
	t.Parallel()
	r := newReliableReceiver(netcore.ReliableUnordered)
	d, dup := r.receive(5, []byte("a"))
	if dup || len(d) != 1 || string(d[0]) != "a" {
		t.Errorf("first receive = %v dup=%v", d, dup)
	}
	// Duplicate of 5.
	if _, dup := r.receive(5, []byte("a")); !dup {
		t.Error("re-receiving seq 5 should be a duplicate")
	}
	// Newer seq delivered immediately (unordered).
	if d, dup := r.receive(8, []byte("b")); dup || len(d) != 1 {
		t.Errorf("receive 8 = %v dup=%v", d, dup)
	}
	// An older, un-seen seq (6) is still new (within window).
	if d, dup := r.receive(6, []byte("c")); dup || len(d) != 1 {
		t.Errorf("receive 6 = %v dup=%v", d, dup)
	}
	// Re-receive 6 → duplicate.
	if _, dup := r.receive(6, []byte("c")); !dup {
		t.Error("re-receiving 6 should be a duplicate")
	}
}

func TestReliableReceiverOrdered(t *testing.T) {
	t.Parallel()
	r := newReliableReceiver(netcore.ReliableOrdered)
	// Arrive out of order: 0, then 2, then 1 → release [0],[],[1,2].
	if d, _ := r.receive(0, []byte("zero")); len(d) != 1 || string(d[0]) != "zero" {
		t.Fatalf("recv 0 = %v", d)
	}
	if d, _ := r.receive(2, []byte("two")); len(d) != 0 {
		t.Errorf("recv 2 (gap) should buffer, got %v", d)
	}
	d, _ := r.receive(1, []byte("one"))
	if len(d) != 2 || string(d[0]) != "one" || string(d[1]) != "two" {
		t.Errorf("recv 1 should release one+two in order, got %v", d)
	}
}
