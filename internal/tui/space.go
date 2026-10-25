package tui

import (
	"fmt"

	"ytmanager/internal/device"
	"ytmanager/internal/engine"
)

func (m model) spaceView(width int) string {
	if !m.spaceKnown {
		return dim.Render("Storage: free space unavailable")
	}
	text := fmt.Sprintf("Storage: %s free / %s", engine.Human(m.space.Available), engine.Human(m.space.Total))
	if m.space.Available < device.SpaceReserve {
		return bad.Render("LOW SPACE · " + text)
	}
	if width >= 60 && m.space.Total > 0 {
		used := 100 * (1 - float64(m.space.Available)/float64(m.space.Total))
		text += fmt.Sprintf(" · %.0f%% used", used)
	}
	return dim.Render(text)
}
