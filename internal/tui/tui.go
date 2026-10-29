// Package tui is the Bubble Tea interface. Each screen has its own file with its key handler and
// view (devices.go, sources.go, settings.go, missing.go); input.go routes keys and the text
// prompt, connection.go watches for unplugged devices, cat.go is the mascot and view.go holds
// styles and shared helpers.
package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/device"
	"ytmanager/internal/engine"
)

type screen int

const (
	screenDevices  screen = iota // pick a drive or phone
	screenBrowse                 // pick a folder inside it
	screenSources                // a destination is open: sources and downloads
	screenSettings               // download settings
	screenLibrary                // search and manage downloaded music
	screenMissing                // unavailable tracks and their alternatives
)

// prompt is what the text input is asking for; promptNone means it is hidden.
type prompt int

const (
	promptNone prompt = iota
	promptPath
	promptMkdir
	promptURLs
	promptAlt
)

type model struct {
	opt     engine.Options
	format  string // -format flag, applied to every destination opened
	screen  screen
	eng     *engine.Engine // nil on screenDevices and screenBrowse
	cursor  int
	library libraryState

	mounts        []device.Mount
	browse        string // folder shown in screenBrowse
	dirs          []string
	browseReturn  bool
	libraryCursor int

	input  textinput.Model
	prompt prompt
	altFor string // track that receives the alternative being typed

	run        run
	lost       *lost           // destination unplugged by accident; reopened when it comes back
	marked     map[string]bool // sources picked with space
	logs       []string
	logScroll  int // wrapped lines above the live log tail; zero follows new messages
	cat        cat
	bar        progress.Model
	width      int
	height     int
	space      device.SpaceInfo
	spaceKnown bool
}

// run is the state of the current sync. Its messages carry ch, so events from a cancelled run
// that arrive after a new one started are recognised and ignored.
type run struct {
	readingURL   string
	readingSince time.Time
	busy         bool
	verb         string // what finished tracks were: downloaded | moved
	urls         []string
	ch           chan tea.Msg
	cancel       context.CancelFunc
	queue        []string // links added just as a sync was ending; started right after it
	jobs         map[string]*job
	order        []string
	total        int // tracks to download in this sync
	done         int
	failed       int
	unavailable  int
}

type job struct {
	sourceID, sourceTitle string
	title                 string
	pct                   float64
}

type (
	eventMsg struct {
		ch chan tea.Msg
		engine.Event
	}
	doneMsg struct {
		ch  chan tea.Msg
		err error
	}
	altMsg struct {
		title, sourceURL string
		err              error
	}
	mountsMsg struct {
		list       []device.Mount
		watched    string // folder that was open when checked
		gone       bool   // it no longer exists: the device was unplugged
		lostPath   string // unplugged destination that was checked
		back       bool   // it exists again: the device was plugged back in
		space      device.SpaceInfo
		spaceKnown bool
	}
)

// Run opens the TUI; with an empty dest it starts on the device picker.
func Run(dest string, opt engine.Options, format string) error {
	in := textinput.New()
	in.Prompt = hi.Render("› ")
	in.CharLimit = 4096
	m := model{
		opt: opt, format: format, input: in, width: 80, height: 24,
		bar:    progress.New(progress.WithGradient("#7c3aed", "#f0abfc"), progress.WithWidth(24)),
		marked: map[string]bool{}, run: run{jobs: map[string]*job{}},
	}
	if dest != "" {
		if err := m.openDest(dest); err != nil {
			return err
		}
	} else {
		m.mounts = device.List()
	}
	final, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	if fm, ok := final.(model); ok && fm.run.cancel != nil {
		fm.run.cancel()
	}
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(catTick(), detect(0, m.watched(), ""))
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case libraryScannedMsg:
		if msg.eng == m.eng {
			m.library.scanning = false
			m.library.cancel = nil
			m.library.all = msg.tracks
			m.library.marked = map[string]bool{}
			m.cursor = 0
			if msg.err != nil {
				m.fail(msg.err)
			}
			m.log(fmt.Sprintf("Scanned %d audio files. External files are review-only.", len(msg.tracks)))
		}
		return m, nil
	case libraryDeletedMsg:
		if msg.eng == m.eng {
			m.library.busy = false
			m.library.all = msg.tracks
			m.library.marked = map[string]bool{}
			m.cursor = 0
			m.log(fmt.Sprintf("Deleted %d track(s).", msg.count))
			if msg.err != nil {
				m.fail(msg.err)
			}
		}
		return m, nil
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.bar.Width = max(10, min(30, msg.Width/4))
		m.input.Width = max(1, msg.Width-6)
		m.scrollLogs(0)
	case tea.MouseMsg:
		return m.mouse(msg)
	case catTickMsg:
		m.cat.frame++
		return m, catTick()
	case mountsMsg:
		m.mounts = msg.list
		if m.eng != nil && msg.watched == m.eng.Root() {
			m.space, m.spaceKnown = msg.space, msg.spaceKnown
		}
		// only if the same folder is still open; otherwise the next check covers the new one
		if msg.gone && msg.watched == m.watched() {
			m.unplugged(msg.watched)
		}
		if m.screen == screenDevices {
			m.cursor = min(m.cursor, len(m.mounts))
		}
		var cmd tea.Cmd
		if msg.back && msg.lostPath == m.lostPath() {
			cmd = m.reconnect()
		}
		return m, tea.Batch(cmd, detect(2*time.Second, m.watched(), m.lostPath()))
	case eventMsg:
		if msg.ch == m.run.ch {
			m.apply(msg.Event)
		}
		return m, wait(msg.ch) // keep draining a stale run so its goroutine can finish
	case doneMsg:
		if msg.ch != m.run.ch {
			return m, nil
		}
		cmd := m.finish(msg.err)
		return m, cmd
	case altMsg:
		cmd := m.altResolved(msg)
		return m, cmd
	case tea.KeyMsg:
		return m.key(msg)
	}
	return m, nil
}

// openDest opens a destination and switches to the sources screen.
func (m *model) openDest(p string) error {
	eng, err := engine.Open(p, m.opt)
	if err != nil {
		return err
	}
	if m.format != "" {
		if err := eng.SetFormat(m.format); err != nil {
			return err
		}
	}
	m.eng, m.screen, m.cursor, m.logs = eng, screenSources, 0, nil
	m.logScroll = 0
	m.library = libraryState{}
	m.browseReturn = false
	m.space, err = device.Space(eng.Root())
	m.spaceKnown = err == nil
	m.lost = nil // opening any destination ends the wait for an unplugged one
	return nil
}

func (m *model) use(p string) {
	if err := m.openDest(p); err != nil {
		m.fail(err)
	}
}
func (m *model) log(s string) {
	if m.logScroll > 0 {
		m.logScroll += len(m.wrapLog(s))
	}
	m.logs = append(m.logs, s)
	if len(m.logs) > logLimit {
		m.logs = m.logs[len(m.logs)-logLimit:]
	}
	m.scrollLogs(0)
}

func (m *model) fail(err error) { m.log(bad.Render("✗ " + err.Error())) }
