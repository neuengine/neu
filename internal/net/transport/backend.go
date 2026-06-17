package transport

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"
)

// SocketBackend abstracts the OS UDP socket so UDPTransport can be driven by
// a deterministic in-memory implementation in tests.
type SocketBackend interface {
	ReadFrom(buf []byte) (n int, addr net.Addr, err error)
	WriteTo(buf []byte, addr net.Addr) error
	SetReadDeadline(t time.Time) error
	LocalAddr() net.Addr
	Close() error
}

// ─── Production backend ──────────────────────────────────────────────────────

// udpConnBackend wraps *net.UDPConn to satisfy SocketBackend.
type udpConnBackend struct{ conn *net.UDPConn }

// newUDPConnBackend binds a UDP socket on addr (e.g., "0.0.0.0:7777" or ":0").
func newUDPConnBackend(addr string) (*udpConnBackend, error) {
	ua, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, fmt.Errorf("transport: resolve %q: %w", addr, err)
	}
	conn, err := net.ListenUDP("udp", ua)
	if err != nil {
		return nil, fmt.Errorf("transport: listen %q: %w", addr, err)
	}
	return &udpConnBackend{conn: conn}, nil
}

func (b *udpConnBackend) ReadFrom(buf []byte) (int, net.Addr, error) {
	n, a, err := b.conn.ReadFromUDP(buf)
	return n, a, err
}

func (b *udpConnBackend) WriteTo(buf []byte, addr net.Addr) error {
	_, err := b.conn.WriteTo(buf, addr)
	return err
}

func (b *udpConnBackend) SetReadDeadline(t time.Time) error { return b.conn.SetReadDeadline(t) }
func (b *udpConnBackend) LocalAddr() net.Addr               { return b.conn.LocalAddr() }
func (b *udpConnBackend) Close() error                      { return b.conn.Close() }

// ─── In-memory test backend ──────────────────────────────────────────────────

// memAddr is a fake net.Addr used by memBackend.
type memAddr string

func (a memAddr) Network() string { return "mem" }
func (a memAddr) String() string  { return string(a) }

// memPacket is a buffered datagram inside memBackend.
type memPacket struct {
	data []byte
	from net.Addr
}

// memBackend is an in-memory SocketBackend for deterministic unit tests.
// Create a linked pair with newMemPair — writes to A appear at B and vice versa.
// LossRate and ReorderRate (0.0–1.0) are applied per WriteTo.
type memBackend struct {
	inbox    chan memPacket
	closedCh chan struct{}
	closeOnce sync.Once

	peer *memBackend
	addr memAddr

	mu       sync.Mutex
	deadline time.Time
	held     *memPacket // one-packet reorder buffer

	LossRate    float64
	ReorderRate float64
	rng         *rand.Rand
}

// newMemPair creates two linked memBackends. seed controls deterministic loss/reorder.
func newMemPair(addrA, addrB string, seed int64) (*memBackend, *memBackend) {
	a := &memBackend{
		inbox:    make(chan memPacket, 1024),
		closedCh: make(chan struct{}),
		addr:     memAddr(addrA),
		rng:      rand.New(rand.NewSource(seed)),
	}
	b := &memBackend{
		inbox:    make(chan memPacket, 1024),
		closedCh: make(chan struct{}),
		addr:     memAddr(addrB),
		rng:      rand.New(rand.NewSource(seed + 1)),
	}
	a.peer = b
	b.peer = a
	return a, b
}

func (b *memBackend) deliver(pkt memPacket) {
	select {
	case b.inbox <- pkt:
	default: // drop on overflow (simulates backpressure)
	}
}

func (b *memBackend) WriteTo(buf []byte, _ net.Addr) error {
	b.mu.Lock()
	select {
	case <-b.closedCh:
		b.mu.Unlock()
		return fmt.Errorf("transport: memBackend closed")
	default:
	}
	if b.LossRate > 0 && b.rng.Float64() < b.LossRate {
		b.mu.Unlock()
		return nil
	}
	reorder := b.ReorderRate > 0 && b.rng.Float64() < b.ReorderRate
	cp := make([]byte, len(buf))
	copy(cp, buf)
	pkt := memPacket{data: cp, from: b.addr}
	if reorder {
		// Hold current; deliver previously held.
		prev := b.held
		b.held = &pkt
		b.mu.Unlock()
		if prev != nil {
			b.peer.deliver(*prev)
		}
		return nil
	}
	// Flush any held packet before delivering current.
	held := b.held
	b.held = nil
	b.mu.Unlock()
	if held != nil {
		b.peer.deliver(*held)
	}
	b.peer.deliver(pkt)
	return nil
}

func (b *memBackend) ReadFrom(buf []byte) (int, net.Addr, error) {
	b.mu.Lock()
	dl := b.deadline
	b.mu.Unlock()

	var timer <-chan time.Time
	if !dl.IsZero() {
		d := time.Until(dl)
		if d <= 0 {
			return 0, nil, &memTimeoutError{}
		}
		t := time.NewTimer(d)
		defer t.Stop()
		timer = t.C
	}

	select {
	case pkt := <-b.inbox:
		n := copy(buf, pkt.data)
		return n, pkt.from, nil
	case <-timer:
		return 0, nil, &memTimeoutError{}
	case <-b.closedCh:
		return 0, nil, fmt.Errorf("transport: memBackend closed")
	}
}

func (b *memBackend) SetReadDeadline(t time.Time) error {
	b.mu.Lock()
	b.deadline = t
	b.mu.Unlock()
	return nil
}

func (b *memBackend) LocalAddr() net.Addr { return b.addr }

func (b *memBackend) Close() error {
	b.closeOnce.Do(func() { close(b.closedCh) })
	return nil
}

// memTimeoutError satisfies net.Error for deadline simulation.
type memTimeoutError struct{}

func (*memTimeoutError) Error() string   { return "transport: i/o timeout" }
func (*memTimeoutError) Timeout() bool   { return true }
func (*memTimeoutError) Temporary() bool { return true }
