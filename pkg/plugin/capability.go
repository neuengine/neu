package plugin

import "strings"

// Capability is a permission string a plugin requests in its manifest (L1 §4.3),
// e.g. "network.outbound" or "events.publish.assistant.*". The engine grants
// capabilities by user approval and enforces them at runtime (INV-3).
type Capability string

// Built-in capability vocabulary (L1 §4.3).
const (
	CapWorldRead        Capability = "world.read"
	CapWorldCommands    Capability = "world.commands"
	CapWorldResources   Capability = "world.resources.write"
	CapFSReadProject    Capability = "fs.read.project"
	CapFSWriteProject   Capability = "fs.write.project"
	CapFSReadUser       Capability = "fs.read.user"
	CapFSWriteUser      Capability = "fs.write.user"
	CapNetworkOutbound  Capability = "network.outbound"
	CapNetworkInbound   Capability = "network.inbound"
	CapProcessSpawn     Capability = "process.spawn"
	CapEditorUI         Capability = "editor.ui"
	CapAssetsRead       Capability = "assets.read"
	CapAssetsWrite      Capability = "assets.write"
	CapCodegen          Capability = "codegen"
	CapTimeRealtime     Capability = "time.realtime"
	CapMetricsPublish   Capability = "metrics.publish"
)

// Tier is how a capability is approved.
type Tier uint8

const (
	// TierGranted is auto-allowed (low-risk, observable).
	TierGranted Tier = iota
	// TierPrompt is decided once per install and remembered.
	TierPrompt
	// TierAlwaysPrompt triggers a confirmation every session.
	TierAlwaysPrompt
)

// defaultTiers maps built-in capabilities to their approval tier (L1 §4.3).
// Unknown / custom capabilities default to TierAlwaysPrompt.
var defaultTiers = map[Capability]Tier{
	CapWorldRead:      TierGranted,
	CapTimeRealtime:   TierGranted,
	CapMetricsPublish: TierGranted,
	CapWorldCommands:  TierPrompt,
	CapWorldResources: TierPrompt,
	CapFSReadProject:  TierPrompt,
	CapFSWriteProject: TierPrompt,
	CapNetworkOutbound: TierPrompt,
	CapEditorUI:       TierPrompt,
	CapAssetsRead:     TierPrompt,
	CapAssetsWrite:    TierPrompt,
	CapFSReadUser:     TierAlwaysPrompt,
	CapFSWriteUser:    TierAlwaysPrompt,
	CapNetworkInbound: TierAlwaysPrompt,
	CapProcessSpawn:   TierAlwaysPrompt,
	CapCodegen:        TierAlwaysPrompt,
}

// DefaultTier returns a capability's approval tier. Topic-scoped capabilities
// (events.publish.*, events.subscribe.*) are TierPrompt; unknown custom
// capabilities are TierAlwaysPrompt (L1 §4.3).
func DefaultTier(c Capability) Tier {
	if t, ok := defaultTiers[c]; ok {
		return t
	}
	if strings.HasPrefix(string(c), "events.publish.") || strings.HasPrefix(string(c), "events.subscribe.") {
		return TierPrompt
	}
	return TierAlwaysPrompt
}

// CapabilitySet is the set of capabilities granted to a plugin.
type CapabilitySet map[Capability]struct{}

// NewCapabilitySet builds a set from a list.
func NewCapabilitySet(caps ...Capability) CapabilitySet {
	s := make(CapabilitySet, len(caps))
	for _, c := range caps {
		s[c] = struct{}{}
	}
	return s
}

// Has reports whether c is granted. A topic capability granted as a wildcard
// (e.g. "events.publish.*") covers any matching specific topic.
func (s CapabilitySet) Has(c Capability) bool {
	if _, ok := s[c]; ok {
		return true
	}
	// Wildcard match: a granted "prefix.*" covers "prefix.anything".
	for granted := range s {
		if g := string(granted); strings.HasSuffix(g, "*") {
			if strings.HasPrefix(string(c), strings.TrimSuffix(g, "*")) {
				return true
			}
		}
	}
	return false
}

// Add inserts a capability.
func (s CapabilitySet) Add(c Capability) { s[c] = struct{}{} }
