package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/engine"
)

func (m *model) togglePause() {
	switch {
	case !m.run.busy:
	case m.eng.Paused():
		m.eng.Resume()
		m.log(dim.Render("Resumed."))
	default:
		m.eng.Pause()
		m.log(dim.Render("Paused. Downloads continue where they stopped when you resume (p)."))
	}
}

// add downloads the links now; during a sync they join it without stopping anything.
func (m *model) add(words []string) tea.Cmd {
	var urls []string
	for _, w := range words {
		if strings.Contains(w, "://") {
			urls = append(urls, w)
		}
	}
	switch {
	case len(urls) == 0:
		m.log(bad.Render("✗ that does not look like a YouTube link"))
	case m.run.busy && m.eng.Add(urls):
		m.run.urls = append(m.run.urls, urls...) // so a reconnect resumes them too
		m.log(dim.Render(fmt.Sprintf("Added %d link(s) to the current download.", len(urls))))
	case m.run.busy: // the sync is wrapping up; start these right after it
		m.run.queue = append(m.run.queue, urls...)
	default:
		return m.sync(urls)
	}
	return nil
}

func (m *model) sync(urls []string) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 256)
	m.run = run{busy: true, urls: urls, ch: ch, cancel: cancel, queue: m.run.queue, jobs: map[string]*job{}}
	eng := m.eng
	go func() {
		err := eng.Sync(ctx, urls, func(e engine.Event) { ch <- eventMsg{ch, e} })
		ch <- doneMsg{ch, err}
	}()
	return wait(ch)
}

func wait(ch chan tea.Msg) tea.Cmd { return func() tea.Msg { return <-ch } }

func (m *model) apply(e engine.Event) {
	r := &m.run
	switch e.Kind {
	case engine.EvLog:
		m.log(e.Msg)
	case engine.EvPlan:
		r.total = e.Total
	case engine.EvStart:
		r.jobs[e.ID] = &job{title: e.Title}
		r.order = append(r.order, e.ID)
	case engine.EvProgress:
		if j := r.jobs[e.ID]; j != nil {
			j.pct = e.Pct
		}
	case engine.EvDone, engine.EvFailed, engine.EvUnavailable, engine.EvStopped:
		switch e.Kind {
		case engine.EvDone:
			r.done++
		case engine.EvFailed:
			r.failed++
			m.log(bad.Render("✗ " + e.Title + ": " + e.Msg))
		case engine.EvUnavailable:
			r.unavailable++
			m.log(dim.Render("- " + e.Title + ": unavailable (n to set an alternative)"))
		}
		delete(r.jobs, e.ID)
		for i, id := range r.order {
			if id == e.ID {
				r.order = append(r.order[:i], r.order[i+1:]...)
				break
			}
		}
	}
}

// finish reports how the sync ended and starts the queued links, if any.
func (m *model) finish(err error) tea.Cmd {
	r := m.run
	if m.eng != nil {
		m.eng.Resume() // never leave the next sync paused
	}
	m.run = run{jobs: map[string]*job{}, queue: r.queue}
	switch {
	case err == context.Canceled:
		m.run.queue = nil
		m.log(dim.Render("Cancelled. Whatever is left stays pending in data.music."))
	case err != nil:
		m.fail(err)
	default:
		msg := fmt.Sprintf("✓ Done: %d downloaded, %d errors", r.done, r.failed)
		if r.unavailable > 0 {
			msg += fmt.Sprintf(", %d unavailable (n)", r.unavailable)
		}
		m.log(ok.Render(msg))
		m.cat.celebrate(r.failed > 0, time.Now())
	}
	if len(m.run.queue) > 0 && m.eng != nil {
		q := m.run.queue
		m.run.queue = nil
		return m.sync(q)
	}
	return nil
}

// progress is how much of this sync is finished, 0..1, counting partial downloads.
func (r run) progress() float64 {
	if r.total == 0 {
		return 0
	}
	p := float64(r.done + r.failed + r.unavailable)
	for _, j := range r.jobs {
		p += j.pct
	}
	return min(1, p/float64(r.total))
}
