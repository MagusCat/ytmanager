package engine

import (
	"context"
	"testing"
	"time"
)

// Pause cancels the track in flight, wait blocks until Resume, and cancelling ctx frees a paused worker.
func TestGate(t *testing.T) {
	g := newGate()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tctx, release, err := g.wait(ctx)
	if err != nil {
		t.Fatal(err)
	}
	g.pause()
	select {
	case <-tctx.Done():
	case <-time.After(time.Second):
		t.Fatal("pause did not cancel the track in flight")
	}
	release()

	got := make(chan error, 1)
	go func() {
		_, release, err := g.wait(ctx)
		if err == nil {
			release()
		}
		got <- err
	}()
	select {
	case <-got:
		t.Fatal("wait returned while paused")
	case <-time.After(50 * time.Millisecond):
	}
	g.unpause()
	if err := <-got; err != nil {
		t.Fatalf("wait after resume: %v", err)
	}

	g.pause()
	go func() { _, _, err := g.wait(ctx); got <- err }()
	cancel()
	if err := <-got; err == nil {
		t.Fatal("cancel did not free a paused worker")
	}
}
