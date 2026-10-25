package engine

import (
	"context"
	"sync"
)

// gate pauses downloads: workers wait in gate.wait before each track, and Pause cancels the
// tracks in flight (yt-dlp cannot be suspended portably). Their partial files are kept, so
// after Resume they continue where they stopped.
type gate struct {
	mu     sync.Mutex
	paused bool
	resume chan struct{}   // closed by Resume
	run    context.Context // cancelled by Pause
	stop   context.CancelFunc
}

func newGate() *gate {
	g := &gate{}
	g.run, g.stop = context.WithCancel(context.Background())
	return g
}

func (e *Engine) Pause()       { e.gate.pause() }
func (e *Engine) Resume()      { e.gate.unpause() }
func (e *Engine) Paused() bool { e.gate.mu.Lock(); defer e.gate.mu.Unlock(); return e.gate.paused }

func (g *gate) pause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.paused {
		g.paused, g.resume = true, make(chan struct{})
		g.stop()
	}
}

func (g *gate) unpause() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.paused {
		g.paused = false
		g.run, g.stop = context.WithCancel(context.Background())
		close(g.resume)
	}
}

// wait blocks while paused, then returns a context for one track that ends with ctx or with
// the next Pause. It fails only if ctx ends while waiting.
func (g *gate) wait(ctx context.Context) (context.Context, context.CancelFunc, error) {
	for {
		g.mu.Lock()
		if !g.paused {
			run := g.run
			g.mu.Unlock()
			tctx, cancel := context.WithCancel(ctx)
			unhook := context.AfterFunc(run, cancel)
			return tctx, func() { unhook(); cancel() }, nil
		}
		resume := g.resume
		g.mu.Unlock()
		select {
		case <-resume:
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		}
	}
}
