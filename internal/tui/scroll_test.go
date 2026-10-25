package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ytmanager/internal/manifest"
)

func scrollModel(t *testing.T) model {
	t.Helper()
	root := t.TempDir()
	library, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 100 {
		id := fmt.Sprintf("A%03d", i)
		s := library.Merge("album", id, id, "https://example.com/"+id, nil)
		s.Creator = fmt.Sprintf("Artist%03d", i)
	}
	if err := library.Save(); err != nil {
		t.Fatal(err)
	}
	m := newModel()
	m.width, m.height, m.bar = 80, 24, progress.New()
	if err := m.openDest(root); err != nil {
		t.Fatal(err)
	}
	return m
}

func sized(m model, width, height int) model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return next.(model)
}

func keyType(m model, kind tea.KeyType) model {
	next, _ := m.Update(tea.KeyMsg{Type: kind})
	return next.(model)
}

func assertFits(t *testing.T, m model) string {
	t.Helper()
	view := ansi.Strip(m.View())
	if lipgloss.Height(view) > m.height {
		t.Fatalf("view exceeds %dx%d: %d lines\n%s", m.width, m.height, lipgloss.Height(view), view)
	}
	for _, line := range strings.Split(view, "\n") {
		if lipgloss.Width(line) > m.width {
			t.Fatalf("view exceeds %d columns: %q", m.width, line)
		}
	}
	return view
}

func TestLibraryFitsTerminalAndKeepsSelectionVisible(t *testing.T) {
	m := scrollModel(t)
	for i := range 40 {
		m.log(fmt.Sprintf("error %03d: %s; sync (Enter) to restore it", i, strings.Repeat("音楽", 20)))
	}
	for _, size := range [][2]int{{120, 40}, {80, 24}, {40, 16}, {40, 12}, {30, 12}} {
		for _, busy := range []bool{false, true} {
			for _, prompt := range []bool{false, true} {
				t.Run(fmt.Sprintf("%dx%d/busy=%t/prompt=%t", size[0], size[1], busy, prompt), func(t *testing.T) {
					n := sized(m, size[0], size[1])
					n.run.busy, n.run.verb, n.run.total = busy, "downloaded", 100
					if busy {
						for i := range 8 {
							id := fmt.Sprint(i)
							n.run.jobs[id] = &job{title: strings.Repeat("Long title ", 8)}
							n.run.order = append(n.run.order, id)
						}
					}
					n = keyType(n, tea.KeyEnd)
					if prompt {
						n.ask(promptURLs, "paste links", "test-input")
					}
					view := assertFits(t, n)
					if !strings.Contains(view, "A099") || !strings.Contains(view, "By Artist099") {
						t.Fatalf("selected album or creator hidden:\n%s", view)
					}
					if !strings.Contains(view, "Logs") {
						t.Fatalf("logs hidden:\n%s", view)
					}
					if prompt && !strings.Contains(view, "test-input") {
						t.Fatalf("input hidden:\n%s", view)
					}
				})
			}
		}
	}
}

func TestSourceScrollPreservesIdentity(t *testing.T) {
	m := scrollModel(t)
	m = keyType(m, tea.KeyEnd)
	m = press(m, " ")
	urls := m.pickedURLs(m.eng.Snapshot())
	if m.cursor != 99 || len(urls) != 1 || !strings.HasSuffix(urls[0], "/A099") {
		t.Fatalf("wrong source marked after scrolling: cursor=%d urls=%v", m.cursor, urls)
	}
	if view := assertFits(t, m); !strings.Contains(view, "A099") || strings.Contains(view, "A000") {
		t.Fatalf("wrong list window:\n%s", view)
	}
	m = keyType(m, tea.KeyHome)
	if view := assertFits(t, m); m.cursor != 0 || !strings.Contains(view, "A000") {
		t.Fatalf("Home did not show the first source:\n%s", view)
	}
}

func TestLogsScrollAndStayAnchored(t *testing.T) {
	m := scrollModel(t)
	for i := range 60 {
		m.log(fmt.Sprintf("log-%03d", i))
	}
	if view := assertFits(t, m); !strings.Contains(view, "log-059") || strings.Contains(view, "log-000") {
		t.Fatalf("logs do not follow the tail:\n%s", view)
	}
	m = keyType(m, tea.KeyPgUp)
	before := strings.Split(m.logsView(), "\n")[1:]
	if m.logScroll == 0 {
		t.Fatal("PgUp did not scroll logs")
	}
	m.log("new-log")
	after := strings.Split(m.logsView(), "\n")[1:]
	if strings.Join(before, "\n") != strings.Join(after, "\n") {
		t.Fatal("new messages moved the logs while reading history")
	}
	if m.cursor != 0 {
		t.Fatal("scrolling logs changed the album selection")
	}
	m = keyType(m, tea.KeyCtrlHome)
	if view := assertFits(t, m); !strings.Contains(view, "log-000") {
		t.Fatalf("oldest log inaccessible:\n%s", view)
	}
	m = keyType(m, tea.KeyCtrlEnd)
	if view := assertFits(t, m); m.logScroll != 0 || !strings.Contains(view, "new-log") {
		t.Fatalf("Ctrl+End did not resume following:\n%s", view)
	}
	for i := range logLimit + 5 {
		m.log(fmt.Sprint(i))
	}
	if len(m.logs) != logLimit {
		t.Fatalf("history is unbounded: %d", len(m.logs))
	}
}

func TestWheelTargetsListsOrLogs(t *testing.T) {
	m := scrollModel(t)
	for i := range 30 {
		m.log(fmt.Sprint(i))
	}
	logY := m.height - m.footerHeight(m.help())
	next, _ := m.Update(tea.MouseMsg{Y: logY + 1, Button: tea.MouseButtonWheelUp})
	m = next.(model)
	if m.logScroll == 0 || m.cursor != 0 {
		t.Fatal("wheel over logs moved the album selection")
	}
	offset := m.logScroll
	next, _ = m.Update(tea.MouseMsg{Y: 3, Button: tea.MouseButtonWheelDown})
	m = next.(model)
	if m.cursor != 3 || m.logScroll != offset {
		t.Fatal("wheel over albums scrolled the logs")
	}
	assertFits(t, m)
}
