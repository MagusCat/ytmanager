package engine

import "fmt"

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
