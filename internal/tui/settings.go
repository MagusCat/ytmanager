package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"ytmanager/internal/engine"
)

var (
	browsers = []string{"", "firefox", "chrome", "chromium", "brave", "edge", "vivaldi", "opera"}
	artists  = []string{"", "main", "all"}
	folders  = []string{"", "artist", "flat"}
	settings = []string{"Format", "Parallel downloads", "Browser cookies", "Artists", "Folders", "Video titles", "Move files to new folders", "Download everything again", "Choose destination folder"}
)

const (
	moveFiles         = 6
	reorganize        = 7
	chooseDestination = 8
)

// labels shows the empty default of artists and folders.
var labels = map[string]string{"": "feat. in title", "main": "main only", "all": "all in artist"}
var folderLabels = map[string]string{"": "Artist/Album", "artist": "Artist", "flat": "no folders"}

func (m model) settingsKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	step := 0
	switch k.String() {
	case "enter":
		switch m.cursor {
		case moveFiles:
			return m, m.moveFiles()
		case reorganize:
			return m, m.reorganize()
		case chooseDestination:
			m.changeDestination()
			return m, nil
		}
		m.screen, m.cursor = screenSources, 0
	case "esc", "o":
		m.screen, m.cursor = screenSources, 0
	case "up", "k":
		m.cursor = max(0, m.cursor-1)
	case "down", "j":
		m.cursor = min(len(settings)-1, m.cursor+1)
	case "left", "h":
		step = -1
	case "right", "l", " ":
		step = 1
	}
	if step != 0 {
		m.changeSetting(step)
	}
	return m, nil
}

func (m *model) changeSetting(step int) {
	if m.run.busy {
		m.log(dim.Render("Wait for the download to finish before changing settings."))
		return
	}
	var err error
	switch m.cursor {
	case 0:
		err = m.eng.SetFormat(cycle(formats, m.eng.Format(), step))
	case 1:
		m.opt.Jobs = min(8, max(1, m.opt.Jobs+step))
	case 2:
		m.opt.Cookies = cycle(browsers, m.opt.Cookies, step)
	case 3:
		m.opt.Artists = cycle(artists, m.opt.Artists, step)
	case 4:
		m.opt.Folders = cycle(folders, m.opt.Folders, step)
	case 5:
		m.opt.RawTitles = !m.opt.RawTitles
	case moveFiles, reorganize, chooseDestination:
		return
	}
	if m.cursor > 0 {
		m.eng.SetOptions(m.opt)
		err = engine.SaveOptions(m.opt)
	}
	if err != nil {
		m.fail(err)
	}
}

// moveFiles rearranges the downloaded files to the Folders setting without downloading them.
func (m *model) moveFiles() tea.Cmd {
	if m.run.busy {
		m.log(dim.Render("Wait for the download to finish before moving files."))
		return nil
	}
	m.screen, m.cursor = screenSources, 0
	m.log(dim.Render("Moving downloaded files to " + folderLabels[m.opt.Folders] + "…"))
	return m.rearrange()
}

// reorganize re-downloads the whole library with the current settings and goes back to the
// sources screen to show the progress.
func (m *model) reorganize() tea.Cmd {
	if m.run.busy {
		m.log(dim.Render("Wait for the download to finish before reorganizing."))
		return nil
	}
	var urls []string
	for _, s := range m.eng.Snapshot() {
		urls = append(urls, s.URL)
	}
	n, err := m.eng.Reorganize()
	if err != nil {
		m.fail(err)
		return nil
	}
	m.screen, m.cursor = screenSources, 0
	if n == 0 || len(urls) == 0 {
		m.log(dim.Render("Nothing downloaded yet to reorganize."))
		return nil
	}
	m.log(dim.Render(fmt.Sprintf("Reorganizing %d tracks: each old file is replaced as its new one arrives.", n)))
	return m.sync(urls)
}

// cycle returns the value step positions away from cur in list, wrapping around.
func cycle(list []string, cur string, step int) string {
	for i, v := range list {
		if v == cur {
			return list[(i+step+len(list))%len(list)]
		}
	}
	return list[0]
}

func (m model) settingsView(b *strings.Builder, w int) {
	header(b, dim.Render("download settings"))
	cookies := m.opt.Cookies
	if cookies == "" {
		cookies = "none"
	}
	titles := "clean"
	if m.opt.RawTitles {
		titles = "as uploaded"
	}
	vals := []string{m.eng.Format(), fmt.Sprint(m.opt.Jobs), cookies, labels[m.opt.Artists], folderLabels[m.opt.Folders], titles}
	notes := []string{"saved in this destination's data.music", "1 to 8 at a time", "for private playlists or Liked Music",
		"how collaborations are tagged", "how files are arranged", `drops "(Official Video)" and the like`}
	var rows []string
	for i, s := range settings {
		if i >= moveFiles {
			note := "enter: apply Folders to what is already downloaded, no download"
			if i == reorganize {
				note = "enter: also applies Format, Artists and Video titles"
			} else if i == chooseDestination {
				note = "enter: choose another folder or subfolder"
			}
			if i == moveFiles {
				rows = append(rows, "")
			}
			rows = append(rows, pick(m.cursor == i, hi.Render(s)+"  "+dim.Render(note)))
			continue
		}
		rows = append(rows, pick(m.cursor == i, fmt.Sprintf("%-20s ‹ %s ›  %s", s, hi.Render(vals[i]), dim.Render(notes[i]))))
	}
	selectedRow := m.cursor
	if m.cursor >= moveFiles {
		selectedRow++ // the blank row before the actions is not selectable
	}
	m.listPanel(b, w, rows, selectedRow, 1, 0)
	m.footer(b, m.help())
}
