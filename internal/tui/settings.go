package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/engine"
)

var (
	browsers = []string{"", "firefox", "chrome", "chromium", "brave", "edge", "vivaldi", "opera"}
	settings = []string{"Format", "Parallel downloads", "Browser cookies"}
)

func (m model) settingsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := 0
	switch k.String() {
	case "esc", "o", "enter":
		m.screen, m.cursor = screenSources, 0
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(len(settings)-1, m.cursor+1)
	case "left", "h":
		step = -1
	case "right", "l", " ":
		step = 1
	}
	if step != 0 {
		m.changeSetting(step)
	}
	return m, nil
}

func (m *model) changeSetting(step int) {
	if m.run.busy {
		m.log(dim.Render("Wait for the download to finish before changing settings."))
		return
	}
	var err error
	switch m.cursor {
	case 0:
		err = m.eng.SetFormat(cycle(formats, m.eng.Format(), step))
	case 1:
		m.opt.Jobs = min(8, max(1, m.opt.Jobs+step))
	case 2:
		m.opt.Cookies = cycle(browsers, m.opt.Cookies, step)
	}
	if m.cursor > 0 {
		m.eng.SetOptions(m.opt)
		err = engine.SaveOptions(m.opt)
	}
	if err != nil {
		m.fail(err)
	}
}

// cycle returns the value step positions away from cur in list, wrapping around.
func cycle(list []string, cur string, step int) string {
	for i, v := range list {
		if v == cur {
			return list[(i+step+len(list))%len(list)]
		}
	}
	return list[0]
}

func (m model) settingsView(b *strings.Builder, w int) {
	header(b, dim.Render("download settings"))
	cookies := m.opt.Cookies
	if cookies == "" {
		cookies = "none"
	}
	vals := []string{m.eng.Format(), fmt.Sprint(m.opt.Jobs), cookies}
	notes := []string{"saved in this destination's data.music", "1 to 8 at a time", "for private playlists or Liked Music"}
	var rows []string
	for i, s := range settings {
		rows = append(rows, pick(m.cursor == i, fmt.Sprintf("%-20s ‹ %s ›  %s", s, hi.Render(vals[i]), dim.Render(notes[i]))))
	}
	panel(b, w, rows)
	m.footer(b, "↑↓ move · ←→ change · esc back")
}
