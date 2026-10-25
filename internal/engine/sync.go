package engine

import (
	"context"
	"errors"
	"fmt"
)

// Sync refreshes source listings, after resuming already-known pending tracks.
func (e *Engine) Sync(ctx context.Context, urls []string, emit func(Event)) error {
	return e.runDownloads(ctx, urls, true, emit)
}

// Download resumes the saved queue and adds URLs without re-reading cached
// sources. A source is refreshed only by an explicit Sync.
func (e *Engine) Download(ctx context.Context, urls []string, emit func(Event)) error {
	return e.runDownloads(ctx, urls, false, emit)
}

func (e *Engine) runDownloads(ctx context.Context, urls []string, refresh bool, emit func(Event)) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("a sync is already running")
	}
	skipped, err := e.enqueue(urls, refresh)
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("could not save download queue: %w", err)
	}
	e.running, e.accepting, e.more = true, true, make(chan struct{}, 1)
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running, e.accepting = false, false
		e.mu.Unlock()
	}()

	if len(skipped) > 5 {
		emit(Event{Kind: EvLog, Msg: fmt.Sprintf("Skipped %d completed sources; use sync to refresh them.", len(skipped))})
	} else {
		for _, title := range skipped {
			emit(Event{Kind: EvLog, Msg: "Already downloaded: " + title + "; skipped (use sync to refresh)."})
		}
	}
	syncErr := e.schedule(ctx, emit)

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.m.WritePlaylists(); err != nil {
		if syncErr != nil {
			return fmt.Errorf("%w; could not write playlists: %v", syncErr, err)
		}
		return err
	}
	if err := e.m.Save(); err != nil {
		if syncErr != nil {
			return fmt.Errorf("%w; could not save data.music: %v", syncErr, err)
		}
		return err
	}
	return syncErr
}

// Add persists links, even if the run is just finishing. Its boolean tells the
// caller whether the current scheduler will consume them or a new run is needed.
func (e *Engine) Add(urls []string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, err := e.enqueue(urls, false); err != nil {
		return e.running && e.accepting, err
	}
	if !e.running || !e.accepting {
		return false, nil
	}
	select {
	case e.more <- struct{}{}:
	default: // a wake-up is already pending
	}
	return true, nil
}

// worker runs do on one track (download or move), retrying it after Resume if Pause interrupted it.
func (e *Engine) worker(ctx context.Context, id string, emit func(Event), do func(context.Context, string, func(Event)) bool) {
	for {
		tctx, release, err := e.gate.wait(ctx)
		if err != nil {
			return // cancelled while paused
		}
		done := do(tctx, id, emit)
		release()
		if done || ctx.Err() != nil {
			return
		}
	}
}
