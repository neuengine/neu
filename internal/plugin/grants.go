package plugin

import (
	"encoding/json"
	"slices"

	pkgplugin "github.com/neuengine/neu/pkg/plugin"
)

// GrantStore persists per-plugin capability grant decisions so a TierPrompt
// capability is decided once per install and remembered (INV-9: no silent
// escalation — only explicitly approved caps are stored). The engine wires a
// JSON-file store via [MarshalGrants]/[UnmarshalGrants]; tests use
// [MemoryGrantStore].
type GrantStore interface {
	// Granted returns the persisted grant set for id (empty set, false if none).
	Granted(id pkgplugin.PluginID) (pkgplugin.CapabilitySet, bool)
	// Save persists caps as id's remembered grant set (overwrites).
	Save(id pkgplugin.PluginID, caps pkgplugin.CapabilitySet) error
}

// PromptFunc asks the user to approve capability c for plugin id, returning true
// to grant. The engine wires it to the editor's approval UI; a nil prompt (CI /
// headless) denies every prompted capability.
type PromptFunc func(id pkgplugin.PluginID, c pkgplugin.Capability) bool

// ResolveGrants computes the capability set to grant a plugin from its required
// capabilities and their approval tiers (L1 §4.3):
//
//   - TierGranted     → auto-allowed.
//   - TierPrompt      → reuse a persisted decision, else prompt once and persist.
//   - TierAlwaysPrompt → prompt every session, never persisted.
//
// It returns the granted set and whether every required capability was granted
// (allOK=false ⇒ the caller should not load the plugin). Only the persistable
// TierPrompt grants are written back, and only when they changed.
func ResolveGrants(man pkgplugin.Manifest, store GrantStore, prompt PromptFunc) (granted pkgplugin.CapabilitySet, allOK bool) {
	granted = pkgplugin.NewCapabilitySet()
	allOK = true

	var persisted pkgplugin.CapabilitySet
	if store != nil {
		persisted, _ = store.Granted(man.ID)
	}
	toPersist := pkgplugin.NewCapabilitySet()
	changed := false

	for _, c := range man.RequiredCaps {
		switch pkgplugin.DefaultTier(c) {
		case pkgplugin.TierGranted:
			granted.Add(c)
		case pkgplugin.TierPrompt:
			if persisted.Has(c) {
				granted.Add(c)
				toPersist.Add(c)
				continue
			}
			if prompt != nil && prompt(man.ID, c) {
				granted.Add(c)
				toPersist.Add(c)
				changed = true
			} else {
				allOK = false
			}
		case pkgplugin.TierAlwaysPrompt:
			if prompt != nil && prompt(man.ID, c) {
				granted.Add(c)
			} else {
				allOK = false
			}
		}
	}

	if store != nil && changed && len(toPersist) > 0 {
		_ = store.Save(man.ID, toPersist)
	}
	return granted, allOK
}

// MemoryGrantStore is an in-memory [GrantStore] — the test double and the live
// cache the file store hydrates into.
type MemoryGrantStore struct {
	grants map[pkgplugin.PluginID]pkgplugin.CapabilitySet
}

// NewMemoryGrantStore returns an empty store.
func NewMemoryGrantStore() *MemoryGrantStore {
	return &MemoryGrantStore{grants: make(map[pkgplugin.PluginID]pkgplugin.CapabilitySet)}
}

// Granted implements [GrantStore].
func (s *MemoryGrantStore) Granted(id pkgplugin.PluginID) (pkgplugin.CapabilitySet, bool) {
	g, ok := s.grants[id]
	return g, ok
}

// Save implements [GrantStore]. The set is copied so later caller mutations
// don't bleed into the store.
func (s *MemoryGrantStore) Save(id pkgplugin.PluginID, caps pkgplugin.CapabilitySet) error {
	cp := pkgplugin.NewCapabilitySet()
	for c := range caps {
		cp.Add(c)
	}
	s.grants[id] = cp
	return nil
}

// MarshalGrants serializes a store's grants to deterministic JSON (sorted ids
// and capabilities) so the engine can persist it to a file. Round-trips with
// [UnmarshalGrants].
func MarshalGrants(s *MemoryGrantStore) ([]byte, error) {
	flat := make(map[string][]string, len(s.grants))
	for id, caps := range s.grants {
		list := make([]string, 0, len(caps))
		for c := range caps {
			list = append(list, string(c))
		}
		slices.Sort(list)
		flat[string(id)] = list
	}
	return json.MarshalIndent(flat, "", "  ")
}

// UnmarshalGrants hydrates a store from JSON written by [MarshalGrants].
func UnmarshalGrants(data []byte) (*MemoryGrantStore, error) {
	var flat map[string][]string
	if err := json.Unmarshal(data, &flat); err != nil {
		return nil, err
	}
	s := NewMemoryGrantStore()
	for id, list := range flat {
		caps := pkgplugin.NewCapabilitySet()
		for _, c := range list {
			caps.Add(pkgplugin.Capability(c))
		}
		s.grants[pkgplugin.PluginID(id)] = caps
	}
	return s, nil
}

var _ GrantStore = (*MemoryGrantStore)(nil)
