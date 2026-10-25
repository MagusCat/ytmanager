package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"ytmanager/internal/device"
	"ytmanager/internal/engine"
)

func newModel(mounts ...device.Mount) model {
	return model{input: textinput.New(), marked: map[string]bool{}, run: run{jobs: map[string]*job{}},
		opt: engine.Options{Jobs: 3}, mounts: mounts}
}

func press(m model, keys ...string) model {
	named := map[string]tea.KeyType{"enter": tea.KeyEnter, "down": tea.KeyDown, "right": tea.KeyRight, "esc": tea.KeyEsc}
	for _, k := range keys {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		if t, ok := named[k]; ok {
			msg = tea.KeyMsg{Type: t}
		}
		next, _ := m.Update(msg)
		m = next.(model)
	}
	return m
}

func paste(m model, s string) model {
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
	return next.(model)
}

func TestCutLeft(t *testing.T) {
	if got := cutLeft("abcdefgh", 5); got != "…efgh" {
		t.Errorf("cutLeft = %q", got)
	}
}

func TestFooterWrapsRecoveryInstructions(t *testing.T) {
	m := newModel()
	m.width = 48
	m.log(bad.Render("✗ " + strings.Repeat("音楽", 10) + ": file is missing; sync (Enter) to restore it"))
	var b strings.Builder
	m.footer(&b, "")
	plain := ansi.Strip(b.String())
	if !strings.Contains(strings.Join(strings.Fields(plain), " "), "sync (Enter) to restore it") {
		t.Fatalf("recovery action lost: %s", plain)
	}
	for _, line := range strings.Split(plain, "\n") {
		if lipgloss.Width(line) > m.width-4 {
			t.Errorf("error overflows terminal: %q", line)
		}
	}
}

// Browse a device → subfolder → new folder → use it → settings → paste while busy → change destination.
func TestBrowseSettingsPaste(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir()) // os.UserConfigDir on Windows
	root := t.TempDir()
	os.Mkdir(filepath.Join(root, "A"), 0o755)
	m := newModel(device.Mount{Path: root})

	m = press(m, "enter")
	if m.screen != screenBrowse || m.browse != root || len(m.dirs) != 1 {
		t.Fatalf("browser at %q with %v", m.browse, m.dirs)
	}
	m = press(m, "down", "down", "right", "n", "B", "enter")
	want := filepath.Join(root, "A", "B")
	if m.browse != want {
		t.Fatalf("browse = %q, want %q", m.browse, want)
	}
	m = press(m, "u")
	if m.screen != screenSources || m.eng.Root() != want {
		t.Fatalf("destination not opened at %q", want)
	}

	m = press(m, "o", "right", "down", "right", "esc")
	if m.eng.Format() != "m4a" || m.opt.Jobs != 4 || engine.LoadOptions().Jobs != 4 {
		t.Fatalf("settings: format %s, jobs %d, saved %d", m.eng.Format(), m.opt.Jobs, engine.LoadOptions().Jobs)
	}

	m = press(m, "o", "down", "down", "down", "right", "down", "right", "down", "right", "esc")
	if o := engine.LoadOptions(); o.Artists != "main" || o.Folders != "artist" || !o.RawTitles {
		t.Fatalf("tag settings not saved: %+v", o)
	}
	m = press(m, "o", "down", "down", "down", "down", "down", "down", "down", "down", "enter") // reorganize with nothing downloaded
	if m.screen != screenSources || m.run.busy || !strings.Contains(m.logs[len(m.logs)-1], "Nothing downloaded") {
		t.Fatalf("reorganize: screen %v busy %v", m.screen, m.run.busy)
	}

	m = press(m, "o", "down", "down", "down", "down", "down", "down", "down", "enter") // move files
	if m.screen != screenSources || !m.run.busy || m.run.verb != "moved" {
		t.Fatalf("move files: screen %v busy %v verb %q", m.screen, m.run.busy, m.run.verb)
	}
	if msg, ok := (<-m.run.ch).(eventMsg); !ok || msg.Kind != engine.EvPlan || msg.Total != 0 {
		t.Fatalf("move files: first message %+v", msg)
	}
	next, _ := m.Update(<-m.run.ch) // doneMsg
	m = next.(model)

	m.run.busy = true // pasting during a sync queues the link
	m = paste(m, "https://music.youtube.com/playlist?list=X")
	if len(m.run.queue) != 1 {
		t.Fatalf("queue = %v", m.run.queue)
	}
	m.run.busy = false

	m = press(m, "c")
	if m.eng == nil || m.eng.Root() != want || m.screen != screenBrowse || m.browse != want {
		t.Fatalf("change destination: screen %d browse %q", m.screen, m.browse)
	}
	m = press(m, "esc")
	if m.screen != screenSources || m.eng.Root() != want {
		t.Fatal("Esc did not return to the previous destination")
	}
}

// When the destination disappears (USB or phone unplugged) the TUI goes back to the device list;
// a notice about a folder that is no longer open is ignored.
func TestUnplugged(t *testing.T) {
	root := t.TempDir()
	m := newModel()
	if err := m.openDest(root); err != nil {
		t.Fatal(err)
	}
	next, _ := m.Update(mountsMsg{watched: "/some/other/folder", gone: true})
	if m = next.(model); m.eng == nil {
		t.Fatal("closed the destination because of a stale notice")
	}
	next, _ = m.Update(mountsMsg{watched: root, gone: true})
	if m = next.(model); m.eng != nil || m.screen != screenDevices || len(m.logs) == 0 {
		t.Fatalf("did not return to the device list: screen %d logs %v", m.screen, m.logs)
	}
}

// The cat walks right as the sync progresses and sleeps while paused.
func TestCatView(t *testing.T) {
	m := newModel()
	if err := m.openDest(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	m.run = run{busy: true, total: 4, jobs: map[string]*job{}}
	start := strings.Index(strings.Split(m.catView(80), "\n")[0], "/")
	m.run.done = 4
	end := strings.Index(strings.Split(m.catView(80), "\n")[0], "/")
	if end <= start {
		t.Fatalf("cat did not move: %d -> %d", start, end)
	}
	m.eng.Pause()
	if !strings.Contains(m.catView(80), "paused") {
		t.Fatal("cat is not sleeping while paused")
	}
}

// Unplugging during a sync remembers its links; when the folder is back the destination reopens.
func TestReconnect(t *testing.T) {
	root := t.TempDir()
	m := newModel()
	if err := m.openDest(root); err != nil {
		t.Fatal(err)
	}
	m.run = run{busy: true, urls: []string{"https://a"}, queue: []string{"https://b"}, cancel: func() {}, ch: make(chan tea.Msg), jobs: map[string]*job{}}
	stale := m.run.ch
	next, _ := m.Update(mountsMsg{watched: root, gone: true})
	m = next.(model)
	if m.lost == nil || len(m.lost.urls) != 2 || m.screen != screenDevices {
		t.Fatalf("lost = %+v, screen %d", m.lost, m.screen)
	}
	if next, _ = m.Update(doneMsg{stale, nil}); next.(model).run.busy {
		t.Fatal("stale run changed the state")
	}

	m.lost.urls = nil // resuming would call yt-dlp; reopening is what is under test
	next, _ = m.Update(mountsMsg{lostPath: root, back: true})
	if m = next.(model); m.eng == nil || m.eng.Root() != root || m.lost != nil {
		t.Fatalf("did not reconnect: eng=%v lost=%v", m.eng, m.lost)
	}
}
