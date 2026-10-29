package engine

import "ytmanager/internal/manifest"

type Health struct{ Done, Pending, Failed, Unavailable, Removed int }

func (e *Engine) Health() Health {
	e.mu.Lock()
	defer e.mu.Unlock()
	var h Health
	for _, t := range e.m.Tracks {
		switch t.Status {
		case manifest.StatusDone:
			h.Done++
		case manifest.StatusPending:
			h.Pending++
		case manifest.StatusFailed:
			h.Failed++
		case manifest.StatusUnavailable:
			h.Unavailable++
		case manifest.StatusRemoved:
			h.Removed++
		}
	}
	return h
}
