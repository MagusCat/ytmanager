package engine

import (
	"net/url"
	"strings"

	"ytmanager/internal/manifest"
)

// requestKey recognizes the same YouTube source across music/www/share links.
func requestKey(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if host == "youtube.com" || host == "music.youtube.com" || host == "m.youtube.com" || host == "youtu.be" {
		if id := u.Query().Get("list"); id != "" {
			return "list:" + id
		}
		id := u.Query().Get("v")
		if host == "youtu.be" {
			id = strings.Trim(u.Path, "/")
		} else if id == "" {
			for _, prefix := range []string{"/shorts/", "/embed/"} {
				if rest, ok := strings.CutPrefix(u.Path, prefix); ok {
					id = strings.Split(rest, "/")[0]
				}
			}
		}
		if id != "" {
			return "track:" + id
		}
	}
	u.Fragment = ""
	return u.String()
}

// sourceForURL is called with e.mu held.
func (e *Engine) sourceForURL(raw string) *manifest.Source {
	key := requestKey(raw)
	for _, source := range e.m.Sources {
		prefix := "list:"
		if source.Type == "track" {
			prefix = "track:"
		}
		if key == requestKey(source.URL) || key == prefix+source.ID {
			return source
		}
	}
	return nil
}

func (e *Engine) sourceSettled(source *manifest.Source) bool {
	if source == nil || len(source.Tracks) == 0 {
		return false
	}
	for _, id := range source.Tracks {
		track := e.m.Tracks[id]
		if track == nil || (track.Status != manifest.StatusDone && track.Status != manifest.StatusUnavailable) {
			return false
		}
	}
	return true
}

// enqueue is called under e.mu. Existing requests keep their FIFO position.
func (e *Engine) enqueue(urls []string, refresh bool) ([]string, error) {
	previous := append([]manifest.SourceRequest(nil), e.m.Queue...)
	var skipped []string
	changed := false
	for _, raw := range urls {
		key, found := requestKey(raw), false
		for i := range e.m.Queue {
			if requestKey(e.m.Queue[i].URL) == key {
				if refresh && !e.m.Queue[i].Refresh {
					e.m.Queue[i].Refresh, changed = true, true
				}
				found = true
				break
			}
		}
		if found {
			continue
		}
		if source := e.sourceForURL(raw); !refresh && e.sourceSettled(source) {
			skipped = append(skipped, source.Title)
			continue
		}
		e.m.Queue = append(e.m.Queue, manifest.SourceRequest{URL: raw, Refresh: refresh})
		changed = true
	}
	if changed {
		if err := e.m.Save(); err != nil {
			e.m.Queue = previous
			return skipped, err
		}
	}
	return skipped, nil
}

func (e *Engine) completeRequest(raw string) {
	for i, request := range e.m.Queue {
		if requestKey(request.URL) == requestKey(raw) {
			e.m.Queue = append(e.m.Queue[:i], e.m.Queue[i+1:]...)
			return
		}
	}
}

// nextRequest also closes the Add race: after it marks the run idle, Add tells
// its caller to start another run, while its request is already persisted.
func (e *Engine) nextRequest(attempted map[string]bool, idle bool) (manifest.SourceRequest, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, request := range e.m.Queue {
		if !attempted[requestKey(request.URL)] {
			return request, true
		}
	}
	if idle {
		e.accepting = false
	}
	return manifest.SourceRequest{}, false
}

func (e *Engine) pendingSources() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	var ids []string
	for _, source := range e.m.Sources {
		for _, id := range source.Tracks {
			if track := e.m.Tracks[id]; track != nil && track.Status != manifest.StatusDone && track.Status != manifest.StatusUnavailable {
				ids = append(ids, source.ID)
				break
			}
		}
	}
	return ids
}

func (e *Engine) HasPending() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.m.Queue) > 0 {
		return true
	}
	for _, source := range e.m.Sources {
		if !e.sourceSettled(source) {
			return true
		}
	}
	return false
}

func (e *Engine) QueuedSources() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.m.Queue)
}
