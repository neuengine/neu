package transport

import (
	"time"

	netcore "github.com/neuengine/neu/internal/net"
)

// seqGreater reports whether a is strictly after b in modular uint16 sequence
// space (handles wraparound: a is "after" b when the forward distance is in the
// first half of the space).
func seqGreater(a, b uint16) bool {
	d := a - b
	return d != 0 && d < 0x8000
}

// ─── RTO estimator (Jacobson/Karels EWMA) ───────────────────────────────────

const (
	minRTO = 200 * time.Millisecond
	maxRTO = 2 * time.Second
)

type rtoEstimator struct {
	srtt   time.Duration
	rttvar time.Duration
	have   bool
}

// sample folds a round-trip measurement into the smoothed estimate.
func (e *rtoEstimator) sample(r time.Duration) {
	if !e.have {
		e.srtt = r
		e.rttvar = r / 2
		e.have = true
		return
	}
	// SRTT = (1-1/8)·SRTT + 1/8·r ; RTTVAR = (1-1/4)·RTTVAR + 1/4·|SRTT-r|
	diff := e.srtt - r
	if diff < 0 {
		diff = -diff
	}
	e.rttvar = (3*e.rttvar + diff) / 4
	e.srtt = (7*e.srtt + r) / 8
}

// rto returns the retransmission timeout, clamped to [minRTO, maxRTO].
func (e *rtoEstimator) rto() time.Duration {
	if !e.have {
		return minRTO
	}
	rto := e.srtt + 4*e.rttvar
	if rto < minRTO {
		return minRTO
	}
	if rto > maxRTO {
		return maxRTO
	}
	return rto
}

// ─── ackTracker: datagram-level received-sequence tracking ───────────────────

// ackTracker records which datagram packet sequences a peer's packets carried,
// producing the piggybacked Ack (Base = highest seen, Bits = bitfield of the 32
// preceding). Wraparound-aware.
type ackTracker struct {
	bits uint32
	base uint16
	any  bool
}

// onReceived folds a received packet sequence into the ACK state.
func (t *ackTracker) onReceived(seq uint16) {
	if !t.any {
		t.base, t.any = seq, true
		return
	}
	switch {
	case seqGreater(seq, t.base):
		shift := seq - t.base
		if shift >= 32 {
			t.bits = 0
		} else {
			t.bits = (t.bits << shift) | (1 << (shift - 1))
		}
		t.base = seq
	case seq != t.base:
		if d := t.base - seq; d <= 32 {
			t.bits |= 1 << (d - 1)
		}
	}
}

// ack returns the piggyback ACK block, or nil if nothing has been received.
func (t *ackTracker) ack() *Ack {
	if !t.any {
		return nil
	}
	return &Ack{Base: t.base, Bits: t.bits}
}

// acked reports whether an Ack covers packet sequence seq.
func acked(a Ack, seq uint16) bool {
	if seq == a.Base {
		return true
	}
	if seqGreater(a.Base, seq) {
		if d := a.Base - seq; d <= 32 {
			return a.Bits&(1<<(d-1)) != 0
		}
	}
	return false
}

// ─── reliableSender (per reliable channel) ───────────────────────────────────

type pendingFrame struct {
	lastSent    time.Time
	payload     []byte
	seq         uint16  // channel-level msg seq; stable across retransmits (for receiver dedup)
	datagramSeq uint16  // datagram PacketSeq of the last transmission (for ACK matching)
	retransmits int
}

// reliableSender holds a sliding window of unACKed frames, retransmits on RTO,
// and samples RTT from ACKs (Karels: never sample a retransmitted frame).
type reliableSender struct {
	window  []pendingFrame
	est     rtoEstimator
	nextSeq uint16
	winCap  int
}

func newReliableSender(windowCap int) *reliableSender {
	if windowCap <= 0 {
		windowCap = 256
	}
	return &reliableSender{winCap: windowCap}
}

// queue assigns the next sequence number to payload and adds it to the window;
// it returns the assigned sequence. The frame is marked unsent (lastSent zero)
// so the first due() emits it.
func (s *reliableSender) queue(payload []byte) (uint16, bool) {
	if len(s.window) >= s.winCap {
		return 0, false // window full — caller must back off
	}
	seq := s.nextSeq
	s.nextSeq++
	cp := make([]byte, len(payload))
	copy(cp, payload)
	s.window = append(s.window, pendingFrame{seq: seq, payload: cp})
	return seq, true
}

// onAck removes acknowledged frames from the window and samples RTT for frames
// acked on their first transmission. Matching uses datagramSeq (the PacketSeq of
// the datagram the frame was last sent in) so channel msg seqs and datagram seqs
// do not need to coincide.
func (s *reliableSender) onAck(a Ack, now time.Time) {
	kept := s.window[:0]
	for i := range s.window {
		f := s.window[i]
		if !f.lastSent.IsZero() && ackFrame(a, f.datagramSeq) {
			if f.retransmits == 0 {
				s.est.sample(now.Sub(f.lastSent))
			}
			continue // acked → drop from window
		}
		kept = append(kept, f)
	}
	s.window = kept
}

// setDatagramSeq records the datagram PacketSeq that a channel msg frame was last
// sent in. Called by sendFrames after Encode so the Ack-matching uses the correct
// datagram sequence number, not the channel-level msg sequence.
func (s *reliableSender) setDatagramSeq(msgSeq, datagramSeq uint16) {
	for i := range s.window {
		if s.window[i].seq == msgSeq {
			s.window[i].datagramSeq = datagramSeq
			return
		}
	}
}

// due returns the frames that must be (re)transmitted now: never-sent frames and
// frames whose RTO has elapsed. It stamps them sent and bumps the retransmit
// count for resends. Returned payloads alias the window — encode immediately.
// datagramSeq is initialised to seq as a placeholder; the caller should call
// setDatagramSeq after it knows the actual datagram PacketSeq.
func (s *reliableSender) due(now time.Time) []Frame {
	rto := s.est.rto()
	var out []Frame
	for i := range s.window {
		f := &s.window[i]
		if f.lastSent.IsZero() || now.Sub(f.lastSent) >= rto {
			if !f.lastSent.IsZero() {
				f.retransmits++
			}
			f.lastSent = now
			f.datagramSeq = f.seq // placeholder; overridden by setDatagramSeq
			out = append(out, Frame{MsgSeq: f.seq, Payload: f.payload})
		}
	}
	return out
}

// pending reports how many frames are awaiting acknowledgement.
func (s *reliableSender) pending() int { return len(s.window) }

// ackFrame reports whether a channel-frame sequence is covered by a datagram
// ACK. (Channel msg sequences and datagram packet sequences share the same
// modular comparison; the sender tracks them per channel.)
func ackFrame(a Ack, seq uint16) bool { return acked(a, seq) }

// ─── reliableReceiver (per channel) ──────────────────────────────────────────

// reliableReceiver deduplicates received frames and, for ReliableOrdered,
// releases them in sequence. A 64-wide sliding bitfield around the highest seq
// detects duplicates; frames older than the window are dropped as duplicates.
type reliableReceiver struct {
	reorder map[uint16][]byte
	seen    uint64
	highest uint16
	expect  uint16
	mode    netcore.DeliveryMode
	started bool
}

func newReliableReceiver(mode netcore.DeliveryMode) *reliableReceiver {
	return &reliableReceiver{mode: mode, reorder: map[uint16][]byte{}}
}

// receive processes a frame and returns the payloads to deliver (in order for
// ReliableOrdered; immediately for ReliableUnordered) plus whether it was a
// duplicate. Unreliable frames are not handled here (the caller delivers them
// directly with a recency check).
func (r *reliableReceiver) receive(seq uint16, payload []byte) (delivered [][]byte, dup bool) {
	if r.isDuplicate(seq) {
		return nil, true
	}
	r.markSeen(seq)

	if r.mode != netcore.ReliableOrdered {
		return [][]byte{payload}, false
	}

	// ReliableOrdered: buffer and release the contiguous run from expect.
	if !r.started {
		r.expect = seq
		r.started = true
	}
	cp := make([]byte, len(payload))
	copy(cp, payload)
	r.reorder[seq] = cp
	for {
		p, ok := r.reorder[r.expect]
		if !ok {
			break
		}
		delivered = append(delivered, p)
		delete(r.reorder, r.expect)
		r.expect++
	}
	return delivered, false
}

// isDuplicate reports whether seq has already been seen. The 64-bit `seen`
// bitfield holds the sequences strictly below `highest` (bit i == highest-(i+1)
// received); `highest` itself is tracked separately. Anything older than the
// window is treated as a duplicate and dropped.
func (r *reliableReceiver) isDuplicate(seq uint16) bool {
	if !r.started {
		return false // the very first frame is always new
	}
	if seqGreater(seq, r.highest) {
		return false // newer than anything seen
	}
	if seq == r.highest {
		return true
	}
	d := r.highest - seq
	if d > 64 {
		return true // too old to verify → assume duplicate
	}
	return r.seen&(1<<(d-1)) != 0
}

// markSeen records seq in the sliding dedup window, advancing `highest` (and
// shifting the bitfield) when seq is newer.
func (r *reliableReceiver) markSeen(seq uint16) {
	if !r.started {
		r.started, r.highest, r.seen = true, seq, 0
		return
	}
	if seqGreater(seq, r.highest) {
		shift := seq - r.highest
		if shift >= 64 {
			r.seen = 0
		} else {
			// Old highest becomes the bit at distance `shift`; existing bits shift up.
			r.seen = (r.seen << shift) | (1 << (shift - 1))
		}
		r.highest = seq
		return
	}
	if d := r.highest - seq; d >= 1 && d <= 64 {
		r.seen |= 1 << (d - 1)
	}
}
