package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

// This animation means the request is outstanding, not that yt-dlp has reported
// progress. The existing UI tick redraws it even when the child produces no output.
func (m model) readingActivity(w int) string {
	if !m.run.busy || m.run.readingURL == "" {
		return ""
	}
	elapsed := max(time.Duration(0), time.Since(m.run.readingSince)).Truncate(time.Second)
	frame := []string{"|", "/", "-", "\\"}[m.cat.frame%4]
	label := fmt.Sprintf("%s Reading link · %s", frame, elapsed)
	if elapsed >= 30*time.Second {
		label = fmt.Sprintf("%s Still waiting · %s", frame, elapsed)
	}
	width := max(1, w-4)
	line := "  " + hi.Render(ansi.Truncate(label, width, "…")) + "\n"
	if m.terminalHeight() >= 24 {
		detail := strings.Join(strings.Fields(m.run.readingURL), " ")
		if elapsed >= 30*time.Second {
			detail = "No result yet; x cancels. " + detail
		}
		line += "  " + dim.Render(ansi.Truncate(detail, width, "…")) + "\n"
	}
	return line
}
