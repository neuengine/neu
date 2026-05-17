package main

import (
	"testing"
)

func TestFramework_GateCriteria(t *testing.T) {
	result, err := runFramework()
	if err != nil {
		t.Fatalf("runFramework error: %v", err)
	}

	if result.ticks < 100 {
		t.Errorf("ticks = %d, want ≥ 100", result.ticks)
	}
	if result.transitions < 2 {
		t.Errorf("transitions = %d, want ≥ 2 (Loading→Game, Game→GameOver)", result.transitions)
	}
	if !result.hierarchyVerified {
		t.Error("3-level hierarchy GlobalTransform propagation was not verified")
	}
	if !result.changedDetected {
		t.Error("Changed[Position] filter did not detect mutation at tick 5")
	}
}
