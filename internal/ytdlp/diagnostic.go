package ytdlp

import (
	"fmt"
	"strings"
)

// diagnostic keeps bounded recent output, including errors without ERROR prefixes.
type diagnostic struct {
	lines    []string
	explicit string
}

func (d *diagnostic) add(line string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "PROG ") || strings.HasPrefix(line, "FILE ") {
		return
	}
	if len(line) > 2048 {
		line = line[:2048] + "…"
	}
	if strings.HasPrefix(line, "ERROR:") {
		d.explicit = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
	}
	d.lines = append(d.lines, line)
	if len(d.lines) > 4 {
		d.lines = d.lines[1:]
	}
}

func processError(err error, detail string) error {
	if detail == "" {
		detail = "the program did not provide a diagnostic message"
	}
	hint := ""
	lower := strings.ToLower(detail)
	switch {
	case strings.Contains(lower, "429"), strings.Contains(lower, "too many requests"):
		hint = "YouTube is limiting requests. Wait, then retry with fewer Parallel tracks/albums."
	case strings.Contains(lower, "sign in"), strings.Contains(lower, "cookies"), strings.Contains(lower, "confirm your age"):
		hint = "Check that this video plays in your browser, then select that browser in settings > Browser cookies."
	case strings.Contains(lower, "403"), strings.Contains(lower, "requested format is not available"):
		hint = "YouTube rejected the audio request. Check your yt-dlp installation for updates, then retry."
	case strings.Contains(lower, "ffmpeg") && (strings.Contains(lower, "not found") || strings.Contains(lower, "not installed")):
		hint = "Install ffmpeg and ensure it is available in PATH."
	case strings.Contains(lower, "permission denied"):
		hint = "Check write permissions and that the destination is connected."
	case strings.Contains(lower, "no space left"):
		hint = "Free space on the destination and temporary disk, then resume."
	case strings.Contains(lower, "timed out"), strings.Contains(lower, "name resolution"), strings.Contains(lower, "connection"):
		hint = "Check the connection, then resume to retry the pending tracks."
	}
	if hint != "" {
		detail += " — " + hint
	}
	return fmt.Errorf("yt-dlp: %s (%w)", detail, err)
}

func (d *diagnostic) detail() string {
	if d.explicit != "" {
		return d.explicit
	}
	return strings.Join(d.lines, " | ")
}
