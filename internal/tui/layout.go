package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func (m model) terminalHeight() int {
	if m.height <= 0 {
		return 24
	}
	return m.height
}

func (m model) terminalWidth() int {
	if m.width <= 0 {
		return 80
	}
	return m.width
}

func (m model) helpLines(help string) []string {
	if m.prompt != promptNone {
		help = "enter confirm · esc cancel"
	}
	w := max(1, m.terminalWidth()-2)
	lines := strings.Split(ansi.Wrap(help, w, ""), "\n")
	if len(lines) > 2 {
		lines = lines[:2]
		lines[1] = ansi.Truncate(lines[1], max(1, w-1), "") + "…"
	}
	return lines
}

func (m model) logHeight() int {
	if len(m.logs) == 0 {
		return 0
	}
	return min(6, max(1, (m.terminalHeight()-12)/3))
}

func (m model) footerHeight(help string) int {
	n := 1 + len(m.helpLines(help))
	if m.prompt != promptNone {
		n += 2
	}
	if h := m.logHeight(); h > 0 {
		n += h + 1 // log label and visible lines
	}
	return n
}

// listPanel keeps whole records around the cursor and reserves the footer and
// any activity below it. group is two for sources (title and creator).
func (m model) listPanel(b *strings.Builder, w int, rows []string, cur, group, after int) {
	available := m.terminalHeight() - m.footerHeight(m.help()) - strings.Count(b.String(), "\n") - after - 2
	visible := max(1, available/group)
	total := (len(rows) + group - 1) / group
	if total > visible {
		indicator := 0
		if available >= group+1 {
			indicator = 1
		}
		visible = max(1, (available-indicator)/group)
		start := min(max(0, cur/group-visible/2), max(0, total-visible))
		end := min(total, start+visible)
		rows = append([]string{}, rows[start*group:min(len(rows), end*group)]...)
		if indicator > 0 {
			rows = append(rows, dim.Render(fmt.Sprintf("  %d-%d of %d · ↑↓ scroll", start+1, end, total)))
		}
	}
	bounded := make([]string, len(rows))
	for i, row := range rows {
		bounded[i] = ansi.Truncate(row, max(1, w-4), "…")
	}
	panel(b, w, bounded)
}

// fitLines prevents terminal wrapping from silently adding rows to the layout.
func fitLines(view string, width, height int) string {
	lines := strings.Split(strings.TrimSuffix(view, "\n"), "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}
