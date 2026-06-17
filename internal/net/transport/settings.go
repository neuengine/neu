package transport

import "time"

// NetworkSettings configures UDPTransport behavior. A zero value is valid;
// withDefaults fills every unset field.
type NetworkSettings struct {
	MaxConnections    int           // 0 → 64
	SendQueueDepth    int           // 0 → 256
	RecvQueueDepth    int           // 0 → 256
	ReadBufSize       int           // 0 → 2048
	ConnectTimeout    time.Duration // 0 → 5s (unused alias; HandshakeTimeout governs)
	HeartbeatInterval time.Duration // 0 → 1s — how often to send keepalive when idle
	DisconnectTimeout time.Duration // 0 → 10s — drop connection if no data in this window
	HandshakeTimeout  time.Duration // 0 → 5s — abort connect if no reply in this window
	MTUProbeTimeout   time.Duration // 0 → 3s — per-step PMTUD probe timeout
}

func (s NetworkSettings) withDefaults() NetworkSettings {
	if s.MaxConnections <= 0 {
		s.MaxConnections = 64
	}
	if s.SendQueueDepth <= 0 {
		s.SendQueueDepth = 256
	}
	if s.RecvQueueDepth <= 0 {
		s.RecvQueueDepth = 256
	}
	if s.ReadBufSize <= 0 {
		s.ReadBufSize = 2048
	}
	if s.ConnectTimeout <= 0 {
		s.ConnectTimeout = 5 * time.Second
	}
	if s.HeartbeatInterval <= 0 {
		s.HeartbeatInterval = time.Second
	}
	if s.DisconnectTimeout <= 0 {
		s.DisconnectTimeout = 10 * time.Second
	}
	if s.HandshakeTimeout <= 0 {
		s.HandshakeTimeout = 5 * time.Second
	}
	if s.MTUProbeTimeout <= 0 {
		s.MTUProbeTimeout = 3 * time.Second
	}
	return s
}
