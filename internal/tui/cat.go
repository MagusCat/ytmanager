package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ytmanager/internal/device"
)

// The mascot: a cat that sits while idle, walks along a track as the sync progresses,
// sleeps while paused and reacts when a sync ends. Every frame is 7 columns wide.

const catWidth = 7

var (
	catStyle = lipgloss.NewStyle().Foreground(accent)
	catHead  = ` /\_/\ `
	catFaces = map[string]string{"open": "( o.o )", "blink": "( -.- )", "happy": "( ^.^ )", "sad": "( ;.; )"}
	catFeet  = map[string]string{"sit": `(")_(")`, "walk1": ` /   \ `, "walk2": `  | |  `}
)

type cat struct {
	frame     int
	moodUntil time.Time // shows the end-of-sync face until then
	sad       bool
}

type catTickMsg struct{}

func catTick() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg { return catTickMsg{} })
}

// celebrate shows a happy (or sad, if something failed) face for a few seconds.
func (c *cat) celebrate(sad bool, now time.Time) {
	c.sad, c.moodUntil = sad, now.Add(6*time.Second)
}

// catView draws the cat and, during a sync, the track it walks along.
func (m model) catView(w int) string {
	f := m.cat.frame
	face, feet, say := "open", "sit", "Paste a link and I will fetch it."
	if f%20 == 0 {
		face = "blink"
	}
	x, track := 0, ""

	if m.run.busy { // walk along the track; while paused the cat sleeps where it stopped
		trackW := min(w-4, 60)
		p := m.run.progress()
		x = int(p * float64(trackW-catWidth))
		filled := min(trackW, x+catWidth/2)
		track = hi.Render(strings.Repeat("━", filled)) + dim.Render(strings.Repeat("─", trackW-filled)) +
			dim.Render(fmt.Sprintf(" %3.0f%%", p*100))
	}
	switch {
	case m.lost != nil:
		face = "sad"
		say = "Waiting for " + cutLeft(device.Label(m.lost.path), 40) + " to come back" + strings.Repeat(".", f/2%4)
		if len(m.lost.urls) > 0 {
			say += " I will resume the download."
		}
	case m.run.busy && m.eng.Paused():
		face = "blink"
		say = "z" + strings.Repeat("Z", f/3%3) + "  paused, p to resume"
	case m.run.busy:
		feet = []string{"walk1", "walk2"}[f/2%2]
		say = fmt.Sprintf("Downloading %d/%d", m.run.done+m.run.failed+m.run.unavailable, m.run.total)
		if m.run.total == 0 {
			say = "Reading the links…"
		}
	case time.Now().Before(m.cat.moodUntil):
		face, say = "happy", "All done!"
		if m.cat.sad {
			face, say = "sad", "Some tracks failed. Check the error messages."
		}
	}

	pad := strings.Repeat(" ", x)
	lines := []string{
		pad + catStyle.Render(catHead),
		pad + catStyle.Render(catFaces[face]) + "  " + dim.Render(say),
		pad + catStyle.Render(catFeet[feet]),
	}
	if track != "" {
		lines = append(lines, track)
	}
	return strings.Join(lines, "\n")
}
