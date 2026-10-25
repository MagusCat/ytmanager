package engine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"ytmanager/internal/device"
)

// schedule finishes known work before reading links, then lists and downloads
// one source at a time. Track workers stay parallel within each source.
func (e *Engine) schedule(parent context.Context, emit func(Event)) error {
	ctx, cancel := context.WithCancelCause(parent)
	defer cancel(nil)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	work, finished := make(chan string), make(chan struct{})
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
			for id := range work {
				e.worker(ctx, id, workerEmit, do)
				finished <- struct{}{}
			}
		}()
	}

	backlog := e.pendingSources()
	if len(backlog) > 0 {
		emit(Event{Kind: EvLog, Msg: fmt.Sprintf("Resuming %d saved source(s) before reading links.", len(backlog))})
	}
	var queue []string
	queued, attempted := map[string]bool{}, map[string]bool{}
	inflight, total, failedReads := 0, 0, 0
	reading := false
	plan := func(ids []string) error {
		add, err := e.plan(ids, queued, emit)
		if err == nil && len(add) > 0 {
			queue, total = append(queue, add...), total+len(add)
			emit(Event{Kind: EvPlan, Total: total})
		}
		return err
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
		if len(queue) == 0 && inflight == 0 && !reading {
			if len(backlog) > 0 {
				id := backlog[0]
				backlog = backlog[1:]
				if err := plan([]string{id}); err != nil {
					cancel(err)
					break
				}
				continue
			}
			request, ok := e.nextRequest(attempted)
			if !ok {
				break
			}
			attempted[requestKey(request.URL)], reading = true, true
			go func() { listed <- e.readSource(ctx, request, workerEmit) }()
		}
		var send chan string
		var next string
		if len(queue) > 0 {
			send, next = work, queue[0]
		}
		select {
		case send <- next:
			queue, inflight = queue[1:], inflight+1
		case <-finished:
			inflight--
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
