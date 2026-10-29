package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"ytmanager/internal/engine"
)

type libraryState struct {
	all                      []engine.LibraryTrack
	marked                   map[string]bool
	query                    string
	duplicates, search, busy bool
	confirm                  []string
	scanning                 bool
	cancel                   context.CancelFunc
}
type libraryScannedMsg struct {
	eng    *engine.Engine
	tracks []engine.LibraryTrack
	err    error
}

type libraryDeletedMsg struct {
	eng    *engine.Engine
	tracks []engine.LibraryTrack
	count  int
	err    error
}

func (m model) libraryRows() []engine.LibraryTrack {
	query := m.library.query
	if m.library.search {
		query = m.input.Value()
	}
	return engine.FilterLibrary(m.library.all, query, m.library.duplicates)
}

func (m model) libraryKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.library.scanning {
		if k.String() == "x" || k.String() == "esc" {
			m.library.cancel()
		}
		return m, nil
	}
	if m.library.busy {
		return m, nil
	}
	if len(m.library.confirm) > 0 {
		switch k.String() {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(len(m.library.confirm)-1, m.cursor+1)
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = len(m.library.confirm) - 1
		case "pgup":
			m.cursor = max(0, m.cursor-max(1, m.terminalHeight()/3))
		case "pgdown":
			m.cursor = min(len(m.library.confirm)-1, m.cursor+max(1, m.terminalHeight()/3))
		case "esc":
			m.library.confirm = nil
		case "enter":
			eng, ids := m.eng, append([]string(nil), m.library.confirm...)
			m.library.busy, m.library.confirm = true, nil
			return m, func() tea.Msg { n, err := eng.DeleteTracks(ids); return libraryDeletedMsg{eng, eng.Library(), n, err} }
		}
		return m, nil
	}
	if m.library.search {
		switch k.String() {
		case "esc":
			m.library.search = false
			m.input.Blur()
			m.cursor = 0
		case "enter":
			m.library.query = m.input.Value()
			m.library.search = false
			m.input.Blur()
			m.cursor = 0
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(k)
			m.cursor = 0
			return m, cmd
		}
		return m, nil
	}
	rows := m.libraryRows()
	switch k.String() {
	case "q":
		return m, tea.Quit
	case "esc", "l":
		m.screen, m.cursor = screenSources, 0
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = max(0, min(len(rows)-1, m.cursor+1))
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = max(0, len(rows)-1)
	case "pgup":
		m.cursor = max(0, m.cursor-max(1, m.terminalHeight()/3))
	case "pgdown":
		m.cursor = max(0, min(len(rows)-1, m.cursor+max(1, m.terminalHeight()/3)))
	case "/":
		m.library.search = true
		m.input.SetValue(m.library.query)
		m.input.Placeholder = "Search title, artist, album or path"
		m.input.CursorEnd()
		return m, m.input.Focus()
	case "s":
		ctx, cancel := context.WithCancel(context.Background())
		m.library.cancel = cancel
		m.library.scanning = true
		eng := m.eng
		return m, func() tea.Msg { rows, err := eng.ScanLibrary(ctx); return libraryScannedMsg{eng, rows, err} }
	case "f":
		m.library.duplicates = !m.library.duplicates
		m.cursor = 0
		m.library.marked = map[string]bool{}
	case " ":
		if len(rows) > 0 {
			id := rows[m.cursor].ID
			m.library.marked[id] = !m.library.marked[id]
		}
	case "a":
		all := len(rows) > 0
		for _, r := range rows {
			all = all && m.library.marked[r.ID]
		}
		for _, r := range rows {
			m.library.marked[r.ID] = !all
		}
	case "d":
		if len(rows) > 0 && rows[m.cursor].Unmanaged {
			m.log("External file: review its path in your file manager. Automatic deletion is limited to managed files.")
		}
		// Only visible marks are actionable; a search cannot hide a deletion target.
		for _, r := range rows {
			if m.library.marked[r.ID] && !r.Unmanaged {
				m.library.confirm = append(m.library.confirm, r.ID)
			}
		}
		if len(m.library.confirm) == 0 && len(rows) > 0 && !rows[m.cursor].Unmanaged {
			m.library.confirm = []string{rows[m.cursor].ID}
		}
		m.cursor = 0
	}
	return m, nil
}

func (m model) libraryView(b *strings.Builder, w int) {
	rows := m.libraryRows()
	if len(m.library.confirm) > 0 {
		// Destructive review takes priority over logs, including at 30x12.
		m.logs = nil
		header(b, fmt.Sprintf("Delete %d?", len(m.library.confirm)))
		b.WriteString(bad.Render("Audio + lyrics. No undo.") + "\n")
		b.WriteString("Skipped by future syncs.\n")
		rows = nil
		byID := map[string]engine.LibraryTrack{}
		for _, r := range m.library.all {
			byID[r.ID] = r
		}
		for _, id := range m.library.confirm {
			rows = append(rows, byID[id])
		}
	} else {
		label := "Library"
		if m.library.scanning {
			label = "Scanning phone files… (x cancel)"
		}
		if m.library.duplicates {
			label = "Possible duplicates"
		}
		header(b, label)
		query := m.library.query
		if m.library.search {
			b.WriteString(ansi.Truncate(m.input.View(), max(1, w), "…") + "\n")
		} else {
			if query == "" {
				query = "all music"
			}
			b.WriteString(dim.Render(ansi.Truncate("Search: "+query, max(1, w), "…")) + "\n")
		}
		marked := 0
		for _, r := range rows {
			if m.library.marked[r.ID] {
				marked++
			}
		}
		b.WriteString(dim.Render(fmt.Sprintf("%d tracks · %d selected · s: scan actual files", len(rows), marked)) + "\n")
		if m.library.duplicates {
			b.WriteString(dim.Render("Similar titles; compare versions.") + "\n")
		}
	}
	var lines []string
	for i, r := range rows {
		mark := "[ ]"
		if m.library.marked[r.ID] {
			mark = "[x]"
		}
		if r.Unmanaged {
			mark = "[external]"
		}
		duration := ""
		if r.Duration > 0 {
			duration = fmt.Sprintf(" · %d:%02d", int(r.Duration)/60, int(r.Duration)%60)
		}
		lines = append(lines, pick(i == m.cursor, mark+" "+strings.Join(strings.Fields(r.Title), " ")+duration))
		lines = append(lines, dim.Render("      "+strings.Join(strings.Fields(r.Path), " ")+" · "+engine.Human(uint64(r.Size))+" · "+r.Note))
	}
	if len(lines) == 0 {
		lines = []string{dim.Render("No matching music. Press / to change your search.")}
	}
	m.listPanel(b, w, lines, m.cursor*2, 2, 0)
	m.footer(b, m.help())
}
