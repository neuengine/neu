package net

import "github.com/neuengine/neu/internal/ecs/world"

// TransportResource wraps a NetworkTransport for storage in the World's
// resource map. Systems retrieve it via world.Resource[TransportResource].
type TransportResource struct {
	T NetworkTransport
}

// InboundQueue holds packets drained from the transport this frame.
// Populated by NetworkReceive (PreUpdate); consumed by replication/RPC systems.
// Cleared at the start of each PreUpdate so stale packets do not carry over.
type InboundQueue struct {
	Packets []InboundPacket
}

// OutboundMessage is a pending send queued by replication/RPC systems.
// Target == 0 means broadcast to all connections.
type OutboundMessage struct {
	Target  ConnectionID
	Channel ChannelID
	Payload []byte
}

// OutboundQueue holds messages to be dispatched by NetworkSend (PostUpdate).
// Replication/RPC systems append to it; NetworkSend drains it.
type OutboundQueue struct {
	Messages []OutboundMessage
}

// networkReceive is the NetworkReceive system body (PreUpdate).
// Drains the transport and stores packets in InboundQueue for the frame.
// Keeps socket I/O off the game loop (l1-networking-system INV-1).
func networkReceive(w *world.World) {
	tr, ok := world.Resource[TransportResource](w)
	if !ok {
		return
	}
	pkts := tr.T.Drain()
	q, qok := world.Resource[InboundQueue](w)
	if !qok {
		return
	}
	q.Packets = pkts
}

// networkSend is the NetworkSend system body (PostUpdate).
// Flushes every OutboundMessage to the transport and resets the queue.
// Keeps socket I/O off the game loop (l1-networking-system INV-1).
func networkSend(w *world.World) {
	tr, ok := world.Resource[TransportResource](w)
	if !ok {
		return
	}
	q, qok := world.Resource[OutboundQueue](w)
	if !qok {
		return
	}
	for _, msg := range q.Messages {
		if msg.Target == 0 {
			tr.T.Broadcast(msg.Channel, msg.Payload)
		} else {
			_ = tr.T.Send(msg.Target, msg.Channel, msg.Payload)
		}
	}
	q.Messages = q.Messages[:0]
}
