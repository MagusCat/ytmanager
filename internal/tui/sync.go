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
	if len(urls) == 0 {
		m.log(bad.Render("✗ that does not look like a YouTube link"))
		return nil
	}
	if m.run.busy {
		active, err := m.eng.Add(urls)
		if err != nil {
			m.fail(fmt.Errorf("could not save links in the queue: %w", err))
			return nil
		}
		if active {
			m.run.urls = append(m.run.urls, urls...)
		} else {
			m.run.queue = append(m.run.queue, urls...)
		}
		m.log(dim.Render(fmt.Sprintf("Saved %d link(s) in the download queue; completed sources are skipped.", len(urls))))
		return nil
	}
	return m.download(urls)
}

func (m *model) download(urls []string) tea.Cmd {
	eng := m.eng
	return m.start(urls, "downloaded", func(ctx context.Context, emit func(engine.Event)) error {
		return eng.Download(ctx, urls, emit)
	})
}

func (m *model) sync(urls []string) tea.Cmd {
	eng := m.eng
	return m.start(urls, "downloaded", func(ctx context.Context, emit func(engine.Event)) error {
		return eng.Sync(ctx, urls, emit)
	})
}

// rearrange moves the downloaded files to the current folder layout.
func (m *model) rearrange() tea.Cmd {
	eng := m.eng
	return m.start(nil, "moved", eng.Rearrange)
}

// start runs a sync or rearrange in the background; urls are resumed if the device reconnects.
func (m *model) start(urls []string, verb string, do func(context.Context, func(engine.Event)) error) tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 256)
	m.run = run{busy: true, verb: verb, urls: urls, ch: ch, cancel: cancel, queue: m.run.queue, jobs: map[string]*job{}}
	go func() {
		err := do(ctx, func(e engine.Event) { ch <- eventMsg{ch, e} })
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
		r.jobs[e.ID] = &job{title: e.Title, sourceID: e.SourceID, sourceTitle: e.SourceTitle}
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
		msg := fmt.Sprintf("✓ Done: %d %s, %d errors", r.done, r.verb, r.failed)
		if r.unavailable > 0 {
			msg += fmt.Sprintf(", %d unavailable (n)", r.unavailable)
		}
		m.log(ok.Render(msg))
		m.cat.celebrate(r.failed > 0, time.Now())
	}
	if len(m.run.queue) > 0 && m.eng != nil {
		q := m.run.queue
		m.run.queue = nil
		return m.download(q)
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
