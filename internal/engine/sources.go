package engine

import (
	"context"
	"fmt"
	"strings"

	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

const cookieHint = " (if it is private, set cookies in settings: o)"

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
		if l.Creator != "" {
			src.Creator = l.Creator
		}
		err = e.m.Save()
		e.mu.Unlock()
		if err != nil {
			emit(Event{Kind: EvLog, Msg: "✗ could not save data.music: " + err.Error(), Err: err})
			continue
		}
		ids = append(ids, src.ID)
		emit(Event{Kind: EvLog, Msg: fmt.Sprintf("✓ %s (%s)", l.Title, tracks(len(entries)))})
	}
	return ids
}
