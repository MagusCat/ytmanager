package tui

func (m *model) changeDestination() {
	if m.run.busy {
		m.log(dim.Render("Wait for the current operation to finish, or cancel it with x, before changing destination."))
		return
	}
	m.browseReturn, m.libraryCursor = true, m.cursor
	if m.screen != screenSources {
		m.libraryCursor = 0
	}
	m.openBrowser(m.eng.Root())
}

func (m *model) returnToLibrary() bool {
	if m.eng == nil || !m.browseReturn {
		return false
	}
	m.screen, m.cursor, m.browseReturn = screenSources, m.libraryCursor, false
	return true
}
