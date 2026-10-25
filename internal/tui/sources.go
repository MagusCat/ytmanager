package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/device"
	"ytmanager/internal/engine"
)

var (
	tags    = map[string]string{"playlist": "PLAYLIST", "album": "ALBUM", "mix": "MIX", "track": "SONG"}
	formats = []string{"opus", "m4a", "mp3"}
)

func (m model) sourcesKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	srcs := m.eng.Snapshot()
	var cmd tea.Cmd
	switch k.String() {
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = max(0, min(len(srcs)-1, m.cursor+1))
	case " ":
		if len(srcs) > 0 {
			id := srcs[m.cursor].ID
			m.marked[id] = !m.marked[id]
		}
	case "a":
		cmd = m.ask(promptURLs, "paste YouTube / YT Music links separated by spaces", "")
	case "enter", "s":
		if !m.run.busy && len(srcs) > 0 {
			cmd = m.sync(m.pickedURLs(srcs))
		}
	case "p":
		m.togglePause()
	case "x":
		if m.run.busy {
			m.run.cancel() // paused workers wake up on cancel too
			m.log(dim.Render("Cancelling…"))
		}
	case "d":
		if !m.run.busy && len(srcs) > 0 {
			if err := m.eng.Remove(srcs[m.cursor].ID); err != nil {
				m.fail(err)
			}
			m.cursor = max(0, min(m.cursor, len(srcs)-2))
		}
	case "o":
		m.screen, m.cursor = screenSettings, 0
	case "n":
		m.screen, m.cursor = screenMissing, 0
	case "c":
		if m.run.busy {
			m.log(dim.Render("Wait for the download to finish before changing destination."))
		} else {
			root := m.eng.Root()
			m.eng, m.marked = nil, map[string]bool{}
			m.openBrowser(root)
		}
	}
	return m, cmd
}

// pickedURLs returns the marked sources, or all of them when none is marked.
func (m model) pickedURLs(srcs []engine.SourceView) []string {
	var picked, all []string
	for _, s := range srcs {
		all = append(all, s.URL)
		if m.marked[s.ID] {
			picked = append(picked, s.URL)
		}
	}
	if len(picked) == 0 {
		return all
	}
	return picked
}

func (m model) sourcesView(b *strings.Builder, w int) {
	info := fmt.Sprintf(" · %s free · %s · %d jobs", engine.Human(device.Free(m.eng.Root())), m.eng.Format(), m.eng.Jobs())
	if m.opt.Cookies != "" {
		info += " · cookies " + m.opt.Cookies
	}
	header(b, cutLeft(device.Label(m.eng.Root()), max(20, w-45))+dim.Render(info))

	var rows []string
	for i, s := range m.eng.Snapshot() {
		mark := dim.Render("○")
		if m.marked[s.ID] {
			mark = hi.Render("●")
		}
		count := fmt.Sprintf("%d/%d", s.Done, s.Total)
		if s.Done+s.Unavailable == s.Total {
			count = ok.Render(count)
		}
		tw := max(10, w-26)
		if s.Unavailable > 0 {
			count += dim.Render(fmt.Sprintf(" · %d unavail.", s.Unavailable))
			tw = max(10, tw-14)
		}
		rows = append(rows, pick(i == m.cursor, fmt.Sprintf("%s %-9s %-*s %s", mark, tags[s.Type], tw, cut(s.Title, tw), count)))
	}
	if len(rows) == 0 {
		rows = []string{dim.Render("No sources yet. Paste a playlist, album or song link to start downloading.")}
	}
	panel(b, w, rows)

	if m.run.busy {
		b.WriteString("\n")
		for _, id := range m.run.order {
			j := m.run.jobs[id]
			b.WriteString(fmt.Sprintf("  %-32s %s\n", cut(j.title, 32), m.bar.ViewAs(j.pct)))
		}
	}
	b.WriteString("\n" + m.catView(w) + "\n")

	help := "paste a link to download it · a add · space mark · enter sync · d remove · n unavailable · o settings · c change destination · q quit"
	if m.run.busy {
		help = "paste a link or a to add more · p pause/resume · x cancel · n unavailable · o settings · q quit"
	}
	m.footer(b, help)
}
