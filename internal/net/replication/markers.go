package replication

import "github.com/neuengine/neu/internal/ecs/component"

// Replicated marks an entity as eligible for replication to remote peers.
// Add this zero-sized tag to any entity whose state must be synchronized.
type Replicated struct{}

// ServerOnly marks a component as visible only on the server side.
// Presence of this tag excludes the component from client-bound payloads.
type ServerOnly struct{}

// ClientOnly marks a component as client-side only.
// Presence of this tag excludes the component from server-bound replication.
type ClientOnly struct{}

// ReplicationRule describes the send policy for a specific component type.
type ReplicationRule uint8

const (
	// RuleReplicate sends the component to peers on every tick it has changed.
	RuleReplicate ReplicationRule = iota
	// RuleServerOnly excludes the component from client-bound payloads.
	RuleServerOnly
	// RuleClientOnly excludes the component from server-bound payloads.
	RuleClientOnly
	// RuleOnChange sends the component only when its value has changed.
	RuleOnChange
	// RuleOnInterval sends the component periodically rather than every tick.
	RuleOnInterval
	// RuleOnEvent sends the component only when a replication event fires.
	RuleOnEvent
)

// ReplicationConfig maps component IDs to their per-type replication rules.
// Systems that build delta-snapshots consult this config to decide which
// components to include and when.
type ReplicationConfig struct {
	rules map[component.ID]ReplicationRule
}

// Set registers or updates the rule for the given component ID.
func (c *ReplicationConfig) Set(id component.ID, rule ReplicationRule) {
	if c.rules == nil {
		c.rules = make(map[component.ID]ReplicationRule)
	}
	c.rules[id] = rule
}

// Get returns the rule for id and whether it has been explicitly configured.
func (c *ReplicationConfig) Get(id component.ID) (ReplicationRule, bool) {
	r, ok := c.rules[id]
	return r, ok
}

// Len returns the number of configured component rules.
func (c *ReplicationConfig) Len() int { return len(c.rules) }
