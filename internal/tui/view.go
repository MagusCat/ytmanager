package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

var (
	accent = lipgloss.Color("#c084fc")
	gray   = lipgloss.Color("#4b5563")
	logo   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#0b0b0f")).Background(accent).Padding(0, 1)
	dim    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6b7280"))
	hi     = lipgloss.NewStyle().Foreground(accent).Bold(true)
	ok     = lipgloss.NewStyle().Foreground(lipgloss.Color("#4ade80"))
	bad    = lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171"))
	box    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(gray).Padding(0, 1)
)

func (m model) View() string {
	var b strings.Builder
	if m.terminalWidth() < 30 || m.terminalHeight() < 12 {
		return fitLines("Enlarge terminal to at least 30x12.", m.terminalWidth(), m.terminalHeight())
	}
	w := m.terminalWidth() - 4
	switch m.screen {
	case screenDevices:
		m.devicesView(&b, w)
	case screenBrowse:
		m.browseView(&b, w)
	case screenSettings:
		m.settingsView(&b, w)
	case screenLibrary:
		m.libraryView(&b, w)
	case screenMissing:
		m.missingView(&b, w)
	default:
		m.sourcesView(&b, w)
	}
	return fitLines(b.String(), m.terminalWidth(), m.terminalHeight())
}

func header(b *strings.Builder, subtitle string) {
	b.WriteString(logo.Render("ytmanager") + " " + subtitle + "\n\n")
}

// steps shows where the user is in device → folder → links.
func steps(cur int) string {
	names := []string{"Device", "Folder", "Links"}
	parts := make([]string, len(names))
	for i, n := range names {
		s := fmt.Sprintf("%d %s", i+1, n)
		if i+1 == cur {
			parts[i] = hi.Render(s)
		} else {
			parts[i] = dim.Render(s)
		}
	}
	return " " + strings.Join(parts, dim.Render("  ›  ")) + "\n\n"
}

// styleHelp highlights the key of each "key label" pair; status text stays plain.
func styleHelp(help string) string {
	parts := strings.Split(help, " · ")
	for i, p := range parts {
		k, label, found := strings.Cut(p, " ")
		if found && len([]rune(k)) <= 7 && strings.ToLower(k) == k {
			parts[i] = hi.Render(k) + " " + dim.Render(label)
		} else {
			parts[i] = dim.Render(p)
		}
	}
	return strings.Join(parts, "   ")
}

func panel(b *strings.Builder, w int, rows []string) {
	b.WriteString(box.Width(w).Render(strings.Join(rows, "\n")) + "\n")
}

// footer writes the recent log, the text input when it is open, and the key help.
func (m model) footer(b *strings.Builder, help string) {
	bodyHeight := max(0, m.terminalHeight()-m.footerHeight(help))
	body := fitLines(b.String(), m.terminalWidth(), max(1, bodyHeight))
	b.Reset()
	if bodyHeight > 0 {
		b.WriteString(body)
		b.WriteString(strings.Repeat("\n", max(0, bodyHeight-strings.Count(body, "\n"))))
	}
	if len(m.logs) > 0 {
		b.WriteString(m.logsView() + "\n")
	}
	if m.prompt != promptNone {
		b.WriteString("\n" + ansi.Truncate(m.input.View(), m.terminalWidth(), "…") + "\n")
	}
	b.WriteString("\n" + strings.Join(m.helpLines(help), "\n"))
}

func pick(on bool, s string) string {
	if on {
		return hi.Render("› ") + s
	}
	return "  " + s
}

func cut(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// cutLeft shortens s to n runes, keeping the end: for paths the last folder matters most.
func cutLeft(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return "…" + string(r[len(r)-n+1:])
}

func trim(s string) string     { return strings.TrimSpace(s) }
func fields(s string) []string { return strings.Fields(s) }
