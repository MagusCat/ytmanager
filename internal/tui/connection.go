package tui

import (
	"os"
	"slices"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/device"
)

// lost remembers an unplugged destination and the links that were syncing, to resume them.
type lost struct {
	path string
	urls []string
}

// detect runs every 2 s off the UI goroutine: mounts phones, refreshes the device list and checks
// that the open folder still exists (os.Stat on a vanished gvfs mount can be slow).
func detect(delay time.Duration, watched, lostPath string) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		device.MountPhones()
		msg := mountsMsg{list: device.List(), watched: watched, lostPath: lostPath}
		if watched != "" {
			_, err := os.Stat(watched)
			msg.gone = err != nil
		}
		if lostPath != "" {
			_, err := os.Stat(lostPath)
			msg.back = err == nil
		}
		return msg
	})
}

func (m model) lostPath() string {
	if m.lost == nil {
		return ""
	}
	return m.lost.path
}

// watched is the folder that must keep existing: the open destination or the browsed folder.
func (m model) watched() string {
	switch {
	case m.eng != nil:
		return m.eng.Root()
	case m.screen == screenBrowse:
		return m.browse
	}
	return ""
}

// unplugged goes back to the device list when the open folder disappears without unmounting,
// remembering what was syncing so reconnect can pick it up.
func (m *model) unplugged(p string) {
	m.lost = &lost{path: p}
	if m.run.busy {
		m.lost.urls = slices.Concat(m.run.urls, m.run.queue)
		m.run.cancel()
	}
	m.run = run{jobs: map[string]*job{}} // detach: the cancelled run's messages are now stale
	m.eng, m.screen, m.cursor, m.prompt = nil, screenDevices, 0, promptNone
	m.marked = map[string]bool{}
	m.input.Reset()
	m.log(bad.Render("✗ " + device.Label(p) + " was disconnected. Plug it back in and it will resume by itself."))
}

// reconnect reopens the lost destination once it is back, and resumes its sync if one was running.
// It only acts from the device screens, so it never replaces a destination the user opened since.
func (m *model) reconnect() tea.Cmd {
	l := m.lost
	if m.eng != nil {
		return nil
	}
	if err := m.openDest(l.path); err != nil {
		return nil // e.g. the phone is still asking for access; the next check retries
	}
	m.lost = nil
	m.log(ok.Render("✓ " + device.Label(l.path) + " is back."))
	if len(l.urls) == 0 {
		return nil
	}
	m.log(dim.Render("Resuming the download…"))
	return m.sync(l.urls)
}
