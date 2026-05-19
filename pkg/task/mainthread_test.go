package task

import (
	"sync"
	"testing"
)

func TestMainThreadExecutor_DrainOrder(t *testing.T) {
	exec := NewMainThreadExecutor()
	exec.Bind()

	var mu sync.Mutex
	var got []int
	for i := range 5 {
		exec.Execute(func() {
			mu.Lock()
			got = append(got, i)
			mu.Unlock()
		})
	}
	exec.PollMainThread()

	if len(got) != 5 {
		t.Fatalf("drained %d items, want 5", len(got))
	}
	for j, v := range got {
		if v != j {
			t.Errorf("got[%d] = %d, want %d (FIFO order violated)", j, v, j)
		}
	}
}

func TestMainThreadExecutor_EmptyPoll(t *testing.T) {
	exec := NewMainThreadExecutor()
	exec.Bind()
	// PollMainThread on an empty queue must return immediately without blocking.
	exec.PollMainThread()
}

// TestMainThreadExecutor_INV3 verifies that PollMainThread panics when called
// from a goroutine that did not call Bind (INV-3).
func TestMainThreadExecutor_INV3(t *testing.T) {
	exec := NewMainThreadExecutor()
	exec.Bind() // bound to THIS test goroutine

	panicked := make(chan bool, 1)
	go func() {
		defer func() {
			panicked <- recover() != nil
		}()
		exec.PollMainThread() // called from a different goroutine → must panic
	}()

	if !<-panicked {
		t.Fatal("expected panic from non-Bind goroutine (INV-3), got none")
	}
}

func TestMainThreadExecutor_UnboundNoPanic(t *testing.T) {
	exec := NewMainThreadExecutor()
	// Bind not called → mainID == 0 → guard skipped → no panic.
	exec.PollMainThread()
}
