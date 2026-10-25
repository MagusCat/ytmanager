package tui

import tea "github.com/charmbracelet/bubbletea"

func (m model) key(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	if k.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.prompt != promptNone {
		return m.promptKey(k)
	}
	if k.Paste {
		cmd := m.paste(string(k.Runes))
		return m, cmd
	}
	if k.String() == "q" {
		return m, tea.Quit
	}
	switch m.screen {
	case screenDevices:
		return m.devicesKey(k)
	case screenBrowse:
		return m.browseKey(k)
	case screenSettings:
		return m.settingsKey(k)
	case screenMissing:
		return m.missingKey(k)
	}
	return m.sourcesKey(k)
}

// paste: on the device screens it is a path, on the sources screen links that download at once,
// on the missing screen the alternative for the selected track.
func (m *model) paste(text string) tea.Cmd {
	switch m.screen {
	case screenDevices, screenBrowse:
		m.use(trim(text))
	case screenSources:
		return m.add(fields(text))
	case screenMissing:
		if list := m.eng.Missing(); len(list) > 0 {
			return m.setAlt(list[m.cursor].ID, trim(text))
		}
	}
	return nil
}

func (m *model) ask(p prompt, placeholder, value string) tea.Cmd {
	m.prompt = p
	m.input.Placeholder = placeholder
	m.input.SetValue(value)
	m.input.CursorEnd()
	return m.input.Focus()
}

func (m model) promptKey(k tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch k.String() {
	case "esc":
		m.prompt = promptNone
		m.input.Reset()
		return m, nil
	case "enter":
	default:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(k)
		return m, cmd
	}
	v, p := trim(m.input.Value()), m.prompt
	m.prompt = promptNone
	m.input.Reset()
	if v == "" {
		return m, nil
	}
	var cmd tea.Cmd
	switch p {
	case promptPath:
		m.use(v)
	case promptMkdir:
		m.mkdir(v)
	case promptURLs:
		cmd = m.add(fields(v))
	case promptAlt:
		cmd = m.setAlt(m.altFor, v)
	}
	return m, cmd
}
