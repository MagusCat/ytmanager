package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/device"
	"ytmanager/internal/engine"
)

// Device list: one row per mount plus "Other path…" at the end.

func (m model) devicesKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch k.String() {
	case "esc":
		m.returnToLibrary()
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(len(m.mounts), m.cursor+1)
	case "r":
		m.mounts = device.List()
		m.cursor = min(m.cursor, len(m.mounts))
	case "x":
		if m.lost != nil {
			m.log(dim.Render("Stopped waiting for " + device.Label(m.lost.path) + "."))
			m.lost = nil
		}
	case "e", "tab": // edit the device path before opening it (e.g. append /Personal)
		v := ""
		if m.cursor < len(m.mounts) {
			v = m.mounts[m.cursor].Path
		}
		cmd = m.ask(promptPath, "/path/to/destination", v)
	case "enter", "right", "l":
		if m.cursor == len(m.mounts) {
			cmd = m.ask(promptPath, "/path/to/destination", "")
		} else {
			m.openBrowser(m.mounts[m.cursor].Path)
		}
	}
	return m, cmd
}

func (m model) devicesView(b *strings.Builder, w int) {
	header(b, dim.Render("choose a destination"))
	pw := max(20, w-20)
	var rows []string
	for i, mt := range m.mounts {
		row := fmt.Sprintf("%-*s %s", pw, cutLeft(device.Label(mt.Path), pw), dim.Render(engine.Human(mt.Free)+" free"))
		rows = append(rows, pick(i == m.cursor, row))
	}
	rows = append(rows, pick(m.cursor == len(m.mounts), "Other path…"))
	activity := ""
	if m.lost != nil && m.terminalHeight() >= 24 {
		activity = "\n" + m.catView(w) + "\n"
	}
	m.listPanel(b, w, rows, m.cursor, 1, strings.Count(activity, "\n"))
	b.WriteString(activity)
	m.footer(b, m.help())
}

// Folder browser: row 0 uses the current folder, row 1 goes up, the rest are subfolders.

// openBrowser shows p or, if it does not exist yet (e.g. Music on a new phone), its closest ancestor.
func (m *model) openBrowser(p string) {
	for {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			break
		}
		parent := filepath.Dir(p)
		if parent == p {
			break
		}
		p = parent
	}
	entries, err := os.ReadDir(p)
	if err != nil {
		m.fail(err)
		return
	}
	m.dirs = m.dirs[:0]
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			m.dirs = append(m.dirs, e.Name())
		}
	}
	m.screen, m.browse, m.cursor = screenBrowse, p, 0
	if !m.browseReturn {
		m.logs, m.logScroll = nil, 0
	}
}

func (m *model) mkdir(name string) {
	p := filepath.Join(m.browse, name)
	if err := os.MkdirAll(p, 0o755); err != nil {
		m.fail(err)
		return
	}
	m.openBrowser(p)
}

func (m model) browseKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch k.String() {
	case "esc":
		if !m.returnToLibrary() {
			m.screen, m.cursor = screenDevices, 0
		}
	case "d":
		m.screen, m.cursor = screenDevices, 0
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(len(m.dirs)+1, m.cursor+1)
	case "left", "h", "backspace":
		m.openBrowser(filepath.Dir(m.browse))
	case "right", "l", "enter":
		switch {
		case m.cursor >= 2:
			m.openBrowser(filepath.Join(m.browse, m.dirs[m.cursor-2]))
		case m.cursor == 1:
			m.openBrowser(filepath.Dir(m.browse))
		case k.String() == "enter":
			m.use(m.browse)
		}
	case "u":
		m.use(m.browse)
	case "n":
		cmd = m.ask(promptMkdir, "new folder name", "")
	case "e", "tab":
		cmd = m.ask(promptPath, "/path/to/destination", m.browse)
	}
	return m, cmd
}

func (m model) browseView(b *strings.Builder, w int) {
	header(b, cutLeft(device.Label(m.browse), max(20, w-12)))
	rows := []string{pick(m.cursor == 0, hi.Render("Use this folder")), pick(m.cursor == 1, dim.Render(".. up"))}
	for i, d := range m.dirs {
		rows = append(rows, pick(m.cursor == i+2, cut(d, w-6)+"/"))
	}
	if len(m.dirs) == 0 {
		rows = append(rows, dim.Render("  (no subfolders: n creates one)"))
	}
	m.listPanel(b, w, rows, m.cursor, 1, 0)
	m.footer(b, m.help())
}
