package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// Unavailable tracks: give each one an alternative (song, album or playlist link) or retry it.

func (m model) missingKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	list := m.eng.Missing()
	var cmd tea.Cmd
	switch k.String() {
	case "esc", "n":
		m.screen, m.cursor = screenSources, 0
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = max(0, min(len(list)-1, m.cursor+1))
	case "enter", "a":
		if len(list) > 0 {
			m.altFor = list[m.cursor].ID
			cmd = m.ask(promptAlt, "song, album or playlist link with “"+cut(list[m.cursor].Title, 40)+"”", "")
		}
	case "r":
		if len(list) > 0 {
			x := list[m.cursor]
			if err := m.eng.Retry(x.ID); err != nil {
				m.fail(err)
				break
			}
			cmd = m.add([]string{x.URL})
			m.cursor = max(0, min(m.cursor, len(list)-2))
		}
	}
	return m, cmd
}

// setAlt resolves the alternative off the UI goroutine (yt-dlp takes a few seconds).
func (m *model) setAlt(id, url string) tea.Cmd {
	eng, src := m.eng, ""
	for _, x := range eng.Missing() {
		if x.ID == id {
			src = x.URL
		}
	}
	m.log(dim.Render("Looking up the alternative…"))
	return func() tea.Msg {
		title, err := eng.SetAlt(context.Background(), id, url)
		return altMsg{title, src, err}
	}
}

// altResolved re-syncs the track's source so the alternative downloads right away.
func (m *model) altResolved(msg altMsg) tea.Cmd {
	if msg.err != nil {
		m.log(bad.Render("✗ alternative: " + msg.err.Error()))
		return nil
	}
	m.log(ok.Render("✓ Alternative: " + msg.title + ". Downloading…"))
	return m.add([]string{msg.sourceURL})
}

func (m model) missingView(b *strings.Builder, w int) {
	list := m.eng.Missing()
	header(b, dim.Render(fmt.Sprintf("%d unavailable", len(list))))
	half := max(10, w/2)
	var rows []string
	for i, x := range list {
		note := x.Source
		if x.Alt != "" {
			note = "the alternative is unavailable too"
		}
		rows = append(rows, pick(i == m.cursor, fmt.Sprintf("%-*s %s", half, cut(x.Title, half), dim.Render(cut(note, max(10, half-6))))))
	}
	if len(rows) == 0 {
		rows = []string{dim.Render("Everything is available.")}
	}
	m.listPanel(b, w, rows, m.cursor, 1, 0)
	m.footer(b, m.help())
}
