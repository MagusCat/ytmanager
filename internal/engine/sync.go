package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"ytmanager/internal/device"
	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

const cookieHint = " (if it is private, set cookies in settings: o)"

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

	e.schedule(ctx, urls, emit)

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.m.WritePlaylists(); err != nil {
		return err
	}
	if err := e.m.Save(); err != nil {
		return err
	}
	return ctx.Err()
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
func (e *Engine) schedule(ctx context.Context, urls []string, emit func(Event)) {
	work := make(chan string)
	finished := make(chan struct{})
	listed := make(chan []string)
	var wg sync.WaitGroup
	for range e.opt.Jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				e.worker(ctx, id, emit)
				finished <- struct{}{}
			}
		}()
	}

	var queue []string
	queued := map[string]bool{}
	inflight, reading, total := 0, 0, 0
	read := func(batch []string) {
		reading++
		go func() { listed <- e.readSources(ctx, batch, emit) }()
	}
	read(urls)

loop:
	for {
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
			if add := e.plan(ids, queued, emit); len(add) > 0 {
				queue, total = append(queue, add...), total+len(add)
				emit(Event{Kind: EvPlan, Total: total})
			}
		case <-e.more:
			if batch := e.takeExtra(false); len(batch) > 0 {
				read(batch)
			}
		case <-ctx.Done():
			break loop
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
}

// worker downloads one track, retrying it after Resume if Pause interrupted it.
func (e *Engine) worker(ctx context.Context, id string, emit func(Event)) {
	for {
		tctx, release, err := e.gate.wait(ctx)
		if err != nil {
			return // cancelled while paused
		}
		done := e.track(tctx, id, emit)
		release()
		if done || ctx.Err() != nil {
			return
		}
	}
}

// plan returns the pending tracks of these sources that are not queued yet, after checking space.
func (e *Engine) plan(sourceIDs []string, queued map[string]bool, emit func(Event)) []string {
	if len(sourceIDs) == 0 {
		return nil // every link failed to read; readSources already said why
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
		need += uint64(d * 32000) // ~256 kbps, an upper bound for every format
	}
	e.mu.Unlock()
	if free := device.Free(e.m.Root); free > 0 && need > free {
		emit(Event{Kind: EvLog, Msg: fmt.Sprintf("✗ not enough space: needs ~%s and %s is free", Human(need), Human(free))})
		return nil
	}
	emit(Event{Kind: EvLog, Msg: fmt.Sprintf("%s to download (~%s)", tracks(len(add)), Human(need))})
	return add
}

func tracks(n int) string {
	if n == 1 {
		return "1 track"
	}
	return fmt.Sprintf("%d tracks", n)
}

// readSources lists every URL with yt-dlp and merges it into the manifest; it returns the source ids.
func (e *Engine) readSources(ctx context.Context, urls []string, emit func(Event)) []string {
	var ids []string
	for _, u := range urls {
		emit(Event{Kind: EvLog, Msg: "Reading " + u})
		l, err := ytdlp.List(ctx, u, e.opt.Cookies)
		switch {
		case ctx.Err() != nil:
			return nil
		case err != nil:
			msg := "✗ " + u + ": " + err.Error()
			if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "cookies") {
				msg += cookieHint
			}
			emit(Event{Kind: EvLog, Msg: msg})
			continue
		case len(l.Entries) == 0:
			emit(Event{Kind: EvLog, Msg: "✗ " + u + ": no tracks" + cookieHint})
			continue
		}
		entries := make([]manifest.Entry, len(l.Entries))
		for i, x := range l.Entries {
			entries[i] = manifest.Entry{ID: x.ID, Title: x.Title, Duration: x.Duration, Unavailable: x.Unavailable}
		}
		e.mu.Lock()
		src := e.m.Merge(l.Type, l.ID, l.Title, u, entries)
		err = e.m.Save()
		e.mu.Unlock()
		if err != nil {
			emit(Event{Kind: EvLog, Msg: "✗ could not save data.music: " + err.Error()})
			continue
		}
		ids = append(ids, src.ID)
		emit(Event{Kind: EvLog, Msg: fmt.Sprintf("✓ %s (%s)", l.Title, tracks(len(entries)))})
	}
	return ids
}
