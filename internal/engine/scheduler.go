package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ytmanager/internal/device"
)

// schedule finishes known work before reading links, then lists and downloads
// up to AlbumJobs sources at a time, sharing a global track-worker limit.
func (e *Engine) schedule(parent context.Context, emit func(Event)) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	type task struct{ id, source, title string }
	work, finished := make(chan task), make(chan task)
	listed := make(chan sourceResult)
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
	do := e.trackWork
	if do == nil {
		do = e.track
	}
	var wg sync.WaitGroup
	for range e.opt.Jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range work {
				e.worker(ctx, item.id, func(event Event) {
					event.SourceID, event.SourceTitle = item.source, item.title
					workerEmit(event)
				}, do)
				finished <- item
			}
		}()
	}

	backlog := e.pendingSources()
	if len(backlog) > 0 {
		emit(Event{Kind: EvLog, Msg: fmt.Sprintf("Resuming %d saved source(s) before reading links.", len(backlog))})
	}
	var queue []task
	active := map[string]int{}
	working := map[string]int{}
	resuming := len(backlog) > 0
	lastSource := ""
	queued, attempted := map[string]bool{}, map[string]bool{}
	inflight, total, failedReads := 0, 0, 0
	reading := false
	plan := func(ids []string) error {
		for _, sourceID := range ids {
			add, err := e.plan([]string{sourceID}, queued, emit)
			if err != nil {
				return err
			}
			if len(add) == 0 {
				continue
			}
			e.mu.Lock()
			title := sourceID
			if source := e.m.Source(sourceID); source != nil {
				title = source.Title
			}
			e.mu.Unlock()
			for _, id := range add {
				queue = append(queue, task{id, sourceID, title})
			}
			active[sourceID] += len(add)
			total += len(add)
			emit(Event{Kind: EvPlan, Total: total})
		}
		return nil
	}

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case err := <-spaceFailure:
			cancel(fmt.Errorf("storage is full; free space and resume: %w", err))
			break loop
		default:
		}
		if resuming && len(backlog) == 0 && len(active) == 0 {
			resuming = false
		}
		if len(active) < e.opt.AlbumJobs && !reading && (!resuming || len(backlog) > 0) {
			if len(backlog) > 0 {
				id := backlog[0]
				backlog = backlog[1:]
				if err := plan([]string{id}); err != nil {
					cancel(err)
					break
				}
				continue
			}
			request, ok := e.nextRequest(attempted, len(active) == 0)
			if !ok && len(active) == 0 {
				break
			}
			if ok {
				attempted[requestKey(request.URL)], reading = true, true
				go func() { listed <- e.readSource(ctx, request, workerEmit) }()
			}
		}
		var send chan task
		var next task
		index := 0
		if len(queue) > 0 {
			for i, item := range queue {
				if working[item.source] < working[queue[index].source] || (working[item.source] == working[queue[index].source] && queue[index].source == lastSource && item.source != lastSource) {
					index = i
				}
			}
			send, next = work, queue[index]
		}
		select {
		case send <- next:
			queue = append(queue[:index], queue[index+1:]...)
			inflight++
			working[next.source]++
			lastSource = next.source
		case item := <-finished:
			inflight--
			working[item.source]--
			active[item.source]--
			if active[item.source] == 0 {
				delete(active, item.source)
			}
		case result := <-listed:
			reading = false
			if result.err != nil {
				failedReads++
			} else if err := plan(result.ids); err != nil {
				cancel(err)
				break loop
			}
		case <-e.more: // new requests are already persisted at the end of the queue
		case <-ctx.Done():
			break loop
		case err := <-spaceFailure:
			cancel(fmt.Errorf("storage is full; free space and resume: %w", err))
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

	close(work)
	workersDone := make(chan struct{})
	go func() { wg.Wait(); close(workersDone) }()
	for reading || workersDone != nil {
		select {
		case <-finished:
		case <-listed:
			reading = false
		case <-workersDone:
			workersDone = nil
		}
	}
	if err := context.Cause(ctx); err != nil {
		return err
	}
	if failedReads > 0 {
		return fmt.Errorf("%d source(s) could not be read; saved in the queue for the next resume", failedReads)
	}
	return nil
}
