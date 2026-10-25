package engine

import (
	"context"
	"fmt"
	"strings"

	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

const cookieHint = " (if it is private, set cookies in settings: o)"

type sourceResult struct {
	ids []string
	err error
}

func (e *Engine) readSource(ctx context.Context, request manifest.SourceRequest, emit func(Event)) sourceResult {
	fail := func(err error) sourceResult {
		if ctx.Err() == nil {
			msg := "✗ " + request.URL + ": " + err.Error()
			if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "cookies") {
				msg += cookieHint
			}
			emit(Event{Kind: EvLog, Msg: msg, Err: err})
		}
		return sourceResult{err: err}
	}

	e.mu.Lock()
	if source := e.sourceForURL(request.URL); !request.Refresh && source != nil && len(source.Tracks) > 0 {
		id, title, settled := source.ID, source.Title, e.sourceSettled(source)
		e.completeRequest(request.URL)
		err := e.m.Save()
		e.mu.Unlock()
		if err != nil {
			return fail(fmt.Errorf("could not save queue: %w", err))
		}
		if settled {
			emit(Event{Kind: EvLog, Msg: "Already downloaded: " + title + "; skipped."})
			return sourceResult{}
		}
		emit(Event{Kind: EvLog, Msg: "Using saved tracks: " + title})
		return sourceResult{ids: []string{id}}
	}
	e.mu.Unlock()

	emit(Event{Kind: EvReading, ID: request.URL})
	defer emit(Event{Kind: EvReadDone, ID: request.URL})
	emit(Event{Kind: EvLog, Msg: "Reading " + request.URL})
	list := e.listSource
	if list == nil {
		list = ytdlp.List
	}
	listing, err := list(ctx, request.URL, e.opt.Cookies)
	if ctx.Err() != nil {
		return sourceResult{err: ctx.Err()}
	}
	if err != nil {
		return fail(err)
	}
	if len(listing.Entries) == 0 || listing.ID == "" {
		return fail(fmt.Errorf("no tracks%s", cookieHint))
	}
	entries := make([]manifest.Entry, len(listing.Entries))
	for i, entry := range listing.Entries {
		entries[i] = manifest.Entry{ID: entry.ID, Title: entry.Title, Duration: entry.Duration, Unavailable: entry.Unavailable}
	}
	e.mu.Lock()
	source := e.m.Merge(listing.Type, listing.ID, listing.Title, request.URL, entries)
	if listing.Creator != "" {
		source.Creator = listing.Creator
	}
	e.completeRequest(request.URL)
	err = e.m.Save()
	id := source.ID
	e.mu.Unlock()
	if err != nil {
		return fail(fmt.Errorf("could not save data.music: %w", err))
	}
	emit(Event{Kind: EvLog, Msg: fmt.Sprintf("Queued %s (%s)", listing.Title, tracks(len(entries)))})
	return sourceResult{ids: []string{id}}
}
