package engine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"ytmanager/internal/device"
)

// Sync reads (or re-reads) every URL, stores the sources in data.music and downloads only what is
// missing. While it runs, Add feeds more links into the same download.
func (e *Engine) Sync(ctx context.Context, urls []string, emit func(Event)) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("a sync is already running")
	}
	e.running, e.extra, e.more = true, nil, make(chan struct{}, 1)
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.running, e.extra = false, nil
		e.mu.Unlock()
	}()

	syncErr := e.schedule(ctx, urls, emit)

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

// Add queues more links into the running sync without stopping it. It returns false when no
// sync is running, so the caller should start one.
func (e *Engine) Add(urls []string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.running {
		return false
	}
	e.extra = append(e.extra, urls...)
	select {
	case e.more <- struct{}{}:
	default: // a wake-up is already pending
	}
	return true
}

// takeExtra returns the links added since the last call. When there are none and idle is true
// it also marks the sync finished, under the same lock, so a concurrent Add is never lost.
func (e *Engine) takeExtra(idle bool) []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	batch := e.extra
	e.extra = nil
	if len(batch) == 0 && idle {
		e.running = false
	}
	return batch
}

// schedule is the sync loop: Jobs workers download from a FIFO queue while new links are read in
// the background and appended to it. It ends when nothing is queued, downloading or being read.
func (e *Engine) schedule(parent context.Context, urls []string, emit func(Event)) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	work := make(chan string)
	finished := make(chan struct{})
	listed := make(chan []string)
	spaceFailure := make(chan error, 1)
	workerEmit := func(event Event) {
		if device.IsSpaceError(event.Err) {
			select {
			case spaceFailure <- event.Err:
			default:
			}
		}
		emit(event)
	}
	var wg sync.WaitGroup
	for range e.opt.Jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				e.worker(ctx, id, workerEmit, e.track)
				finished <- struct{}{}
			}
		}()
	}

	var queue []string
	queued := map[string]bool{}
	inflight, reading, total := 0, 0, 0
	read := func(batch []string) {
		reading++
		go func() { listed <- e.readSources(ctx, batch, workerEmit) }()
	}
	read(urls)

loop:
	for {
		select {
		case err := <-spaceFailure:
			cancel(fmt.Errorf("storage is full; free space and sync to resume: %w", err))
			break loop
		default:
		}
		if len(queue) == 0 && inflight == 0 && reading == 0 {
			batch := e.takeExtra(true)
			if len(batch) == 0 {
				break
			}
			read(batch)
		}
		var send chan string // nil (never ready) while the queue is empty
		var next string
		if len(queue) > 0 {
			send, next = work, queue[0]
		}
		select {
		case send <- next:
			queue, inflight = queue[1:], inflight+1
		case <-finished:
			inflight--
		case ids := <-listed:
			reading--
			add, err := e.plan(ids, queued, emit)
			if err != nil {
				cancel(err)
				break loop
			}
			if len(add) > 0 {
				queue, total = append(queue, add...), total+len(add)
				emit(Event{Kind: EvPlan, Total: total})
			}
		case <-e.more:
			if batch := e.takeExtra(false); len(batch) > 0 {
				read(batch)
			}
		case <-ctx.Done():
			break loop
		case err := <-spaceFailure:
			cancel(fmt.Errorf("storage is full; free space and sync to resume: %w", err))
			break loop
		case <-ticker.C:
			if inflight > 0 || len(queue) > 0 {
				if err := e.checkSpace(0); err != nil {
					cancel(err)
					break loop
				}
			}
		}
	}

	// cancelled or done: let workers and readers finish without blocking on their channels
	close(work)
	workersDone := make(chan struct{})
	go func() { wg.Wait(); close(workersDone) }()
	for reading > 0 || workersDone != nil {
		select {
		case <-finished:
		case <-listed:
			reading--
		case <-workersDone:
			workersDone = nil
		}
	}
	return context.Cause(ctx)
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

// plan returns the pending tracks of these sources that are not queued yet, after checking space.
func (e *Engine) plan(sourceIDs []string, queued map[string]bool, emit func(Event)) ([]string, error) {
	if len(sourceIDs) == 0 {
		return nil, nil // every link failed to read; readSources already said why
	}
	e.mu.Lock()
	var add []string
	var need uint64
	for _, id := range e.m.Pending(sourceIDs) {
		if queued[id] {
			continue
		}
		queued[id] = true
		add = append(add, id)
		d := e.m.Tracks[id].Duration
		if d == 0 {
			d = 240
		}
		need += uint64(d*40000) + 1<<20 // ~320 kbps plus tags and cover art
	}
	e.mu.Unlock()
	if len(add) > 0 {
		if err := e.checkSpace(need); err != nil {
			return nil, err
		}
	}
	emit(Event{Kind: EvLog, Msg: fmt.Sprintf("%s to download (~%s)", tracks(len(add)), Human(need))})
	return add, nil
}

func tracks(n int) string {
	if n == 1 {
		return "1 track"
	}
	return fmt.Sprintf("%d tracks", n)
}
