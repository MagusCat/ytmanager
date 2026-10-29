package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// The welcome banner on the device picker: the cat beside a half-block title.

var (
	bannerFont = map[rune][3]string{
		'Y': {"█ █", "▀█▀", " ▀ "},
		'T': {"▀█▀", " █ ", " ▀ "},
		'M': {"█▀▄▀█", "█ ▀ █", "▀   ▀"},
		'A': {"▄▀▀▄", "█▀▀█", "▀  ▀"},
		'N': {"█▄ █", "█ ▀█", "▀  ▀"},
		'G': {"█▀▀▀", "█ ▀█", "▀▀▀▀"},
		'E': {"█▀▀", "█▀▀", "▀▀▀"},
		'R': {"█▀▀▄", "█▀▀▄", "▀  ▀"},
	}
	bannerColors = []string{"#a78bfa", "#b388fa", "#c084fc", "#cc7ef8", "#d97df5", "#e879f9", "#ee77e0", "#f472c6", "#f472b6"}
)

// splashFits reports whether the terminal has room for the banner above the device list.
func (m model) splashFits(w int) bool { return w >= 56 && m.terminalHeight() >= 22 }

func (m model) splash(w int) string {
	face := catFaces["open"]
	if m.cat.frame%20 == 0 {
		face = catFaces["blink"]
	}
	rows := []string{catStyle.Render(catHead) + "  ", catStyle.Render(face) + "  ", catStyle.Render(catFeet["sit"]) + "  "}
	for i, r := range "YTMANAGER" {
		c := lipgloss.NewStyle().Foreground(lipgloss.Color(bannerColors[i])).Bold(true)
		for j := range rows {
			rows[j] += " " + c.Render(bannerFont[r][j])
		}
	}
	center := func(s string) string { return lipgloss.PlaceHorizontal(w+2, lipgloss.Center, s) + "\n" }
	var b strings.Builder
	b.WriteString("\n")
	for _, r := range rows {
		b.WriteString(center(r))
	}
	b.WriteString("\n" + center(dim.Render("♪  YouTube Music to your phone · organized · offline  ♪")))
	b.WriteString(center(dim.Render(strings.Repeat("─", 44))))
	b.WriteString(center(strings.TrimSpace(steps(1))) + "\n")
	b.WriteString(" " + hi.Render("WHERE SHOULD YOUR MUSIC GO?") + "\n")
	return b.String()
}
