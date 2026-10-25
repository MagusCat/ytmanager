package engine

import "testing"

// Add only works while a sync runs, and the sync only ends when no added link is waiting,
// so a link added at the last moment is never dropped.
func TestAddAndTakeExtra(t *testing.T) {
	e := &Engine{more: make(chan struct{}, 1)}
	if e.Add([]string{"a"}) {
		t.Fatal("Add accepted with no sync running")
	}
	e.running = true
	e.Add([]string{"a"})
	e.Add([]string{"b"}) // second wake-up is coalesced, the link is not
	if got := e.takeExtra(true); len(got) != 2 || !e.running {
		t.Fatalf("takeExtra = %v, running %v", got, e.running)
	}
	if got := e.takeExtra(true); got != nil || e.running {
		t.Fatalf("idle with nothing added should end the sync: %v, running %v", got, e.running)
	}
}
