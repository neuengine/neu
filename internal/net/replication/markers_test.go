package replication

import (
	"testing"

	"github.com/neuengine/neu/internal/ecs/component"
)

func TestReplicationConfigSetGet(t *testing.T) {
	t.Parallel()
	var cfg ReplicationConfig
	cfg.Set(component.ID(1), RuleReplicate)
	cfg.Set(component.ID(2), RuleServerOnly)
	cfg.Set(component.ID(3), RuleOnChange)

	cases := []struct {
		id   component.ID
		want ReplicationRule
	}{
		{1, RuleReplicate},
		{2, RuleServerOnly},
		{3, RuleOnChange},
	}
	for _, tc := range cases {
		r, ok := cfg.Get(tc.id)
		if !ok || r != tc.want {
			t.Errorf("Get(%d) = %v, %v; want %v, true", tc.id, r, ok, tc.want)
		}
	}
	_, ok := cfg.Get(component.ID(99))
	if ok {
		t.Error("Get(99) should return false for unregistered ID")
	}
}

func TestReplicationConfigLen(t *testing.T) {
	t.Parallel()
	var cfg ReplicationConfig
	if cfg.Len() != 0 {
		t.Errorf("Len() on empty config = %d, want 0", cfg.Len())
	}
	cfg.Set(component.ID(1), RuleReplicate)
	cfg.Set(component.ID(2), RuleClientOnly)
	if cfg.Len() != 2 {
		t.Errorf("Len() = %d, want 2", cfg.Len())
	}
}

func TestReplicationConfigOverwrite(t *testing.T) {
	t.Parallel()
	var cfg ReplicationConfig
	cfg.Set(component.ID(5), RuleReplicate)
	cfg.Set(component.ID(5), RuleOnEvent)
	r, ok := cfg.Get(component.ID(5))
	if !ok || r != RuleOnEvent {
		t.Errorf("overwrite: got %v, %v; want RuleOnEvent, true", r, ok)
	}
	if cfg.Len() != 1 {
		t.Errorf("Len() after overwrite = %d, want 1", cfg.Len())
	}
}

func TestRuleConstantsUnique(t *testing.T) {
	t.Parallel()
	all := []ReplicationRule{
		RuleReplicate, RuleServerOnly, RuleClientOnly,
		RuleOnChange, RuleOnInterval, RuleOnEvent,
	}
	seen := make(map[ReplicationRule]bool)
	for _, r := range all {
		if seen[r] {
			t.Errorf("duplicate rule value: %d", r)
		}
		seen[r] = true
	}
	if RuleReplicate != 0 {
		t.Errorf("RuleReplicate = %d, want 0 (iota base)", RuleReplicate)
	}
}
