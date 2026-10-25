package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

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
	case "home":
		m.cursor = 0
	case "end":
		m.cursor = max(0, len(srcs)-1)
	case " ":
		if len(srcs) > 0 {
			id := srcs[m.cursor].ID
			m.marked[id] = !m.marked[id]
		}
	case "a":
		cmd = m.ask(promptURLs, "paste YouTube / YT Music links separated by spaces", "")
	case "enter":
		if !m.run.busy && (len(srcs) > 0 || m.eng.HasPending()) {
			cmd = m.download(m.pickedURLs(srcs))
		}
	case "s":
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
	case "c", "esc", "backspace":
		m.changeDestination()
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
	info := fmt.Sprintf(" · %s · %d jobs", m.eng.Format(), m.eng.Jobs())
	if m.opt.Cookies != "" {
		info += " · cookies " + m.opt.Cookies
	}
	b.WriteString(logo.Render("ytmanager") + " " + cutLeft(device.Label(m.eng.Root()), max(10, w-35)) + dim.Render(info) + "\n")
	b.WriteString(m.spaceView(w) + "\n")

	var rows []string
	for i, s := range m.eng.Snapshot() {
		mark := dim.Render("○")
		if m.marked[s.ID] {
			mark = hi.Render("●")
		}
		active := false
		for _, j := range m.run.jobs {
			if j.sourceID == s.ID {
				active = true
				break
			}
		}
		count := fmt.Sprintf("%d/%d", s.Done, s.Total)
		if s.Done+s.Unavailable == s.Total {
			count = ok.Render(count)
		}
		if s.Unavailable > 0 {
			count += dim.Render(fmt.Sprintf(" · %d unavail.", s.Unavailable))
		}
		if active {
			count = hi.Render("downloading") + " · " + count
		}
		prefix := fmt.Sprintf("%s %-9s ", mark, tags[s.Type])
		tw := max(1, w-7-ansi.StringWidth(prefix)-ansi.StringWidth(count))
		title := ansi.Truncate(strings.Join(strings.Fields(s.Title), " "), tw, "…")
		rows = append(rows, pick(i == m.cursor, prefix+title+strings.Repeat(" ", max(0, tw-ansi.StringWidth(title)))+" "+count))
		origin := s.URL
		if s.Creator != "" {
			origin = "By " + s.Creator
		}
		rows = append(rows, dim.Render("    "+ansi.Truncate(strings.Join(strings.Fields(origin), " "), max(1, w-8), "…")))
	}
	if len(rows) == 0 {
		message := "No sources yet. Paste a playlist, album or song link to start downloading."
		if queued := m.eng.QueuedSources(); queued > 0 {
			message = fmt.Sprintf("%d saved source(s) waiting. Press Enter to resume.", queued)
		}
		rows = []string{dim.Render(message)}
	}
	activity := m.sourceActivity(w)
	m.listPanel(b, w, rows, m.cursor*2, 2, strings.Count(activity, "\n"))
	b.WriteString(activity)
	m.footer(b, m.help())
}

func (m model) sourceActivity(w int) string {
	if m.terminalHeight() < 24 {
		if m.run.busy && (m.terminalHeight() >= 16 || m.prompt == promptNone) {
			return fmt.Sprintf("  %s %d/%d · %d active\n", m.run.verb, m.run.done, m.run.total, len(m.run.jobs)) + m.activeAlbums(w)
		}
		return ""
	}
	var b strings.Builder
	if m.run.busy {
		b.WriteString(m.activeAlbums(w))
		limit := 1
		if m.terminalHeight() >= 32 {
			limit = 3
		}
		for i, id := range m.run.order {
			if i >= limit {
				b.WriteString(dim.Render(fmt.Sprintf("  %d more active tracks", len(m.run.order)-limit)) + "\n")
				break
			}
			if j := m.run.jobs[id]; j != nil {
				title := j.title
				if j.sourceTitle != "" {
					title = j.sourceTitle + ": " + title
				}
				b.WriteString(fmt.Sprintf("  %-32s %s\n", cut(title, 32), m.bar.ViewAs(j.pct)))
			}
		}
	}
	b.WriteString("\n" + m.catView(w) + "\n")
	return b.String()
}

// activeAlbums stays visible even when the source list is scrolled away.
func (m model) activeAlbums(w int) string {
	var names []string
	seen := map[string]bool{}
	for _, id := range m.run.order {
		j := m.run.jobs[id]
		if j != nil && j.sourceTitle != "" && !seen[j.sourceID] {
			seen[j.sourceID] = true
			names = append(names, j.sourceTitle)
		}
	}
	if len(names) == 0 {
		return ""
	}
	label := "Downloading"
	if m.eng != nil && m.eng.Paused() {
		label = "Paused"
	}
	return "  " + hi.Render(ansi.Truncate(label+": "+strings.Join(names, " · "), max(1, w-4), "…")) + "\n"
}
