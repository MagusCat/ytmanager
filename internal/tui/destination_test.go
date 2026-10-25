package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ytmanager/internal/device"
)

func TestDestinationBrowserCanCancelOrChooseSubfolder(t *testing.T) {
	m := scrollModel(t)
	root, engine := m.eng.Root(), m.eng
	if err := os.Mkdir(filepath.Join(root, "Subfolder"), 0o755); err != nil {
		t.Fatal(err)
	}
	m.cursor, m.marked["A025"] = 25, true
	m.log("keep this log")
	m = press(m, "esc")
	if m.screen != screenBrowse || m.eng != engine || m.browse != root {
		t.Fatal("Esc did not open the current destination in the browser")
	}
	m = press(m, "esc")
	if m.screen != screenSources || m.cursor != 25 || !m.marked["A025"] || len(m.logs) != 1 {
		t.Fatal("cancelling lost the library selection or history")
	}
	m = press(m, "c", "down", "down", "right", "u")
	if m.screen != screenSources || m.eng.Root() != filepath.Join(root, "Subfolder") {
		t.Fatalf("subfolder not chosen: %s", m.eng.Root())
	}
	if _, err := os.Stat(filepath.Join(root, "data.music")); err != nil {
		t.Fatal("changing destination changed the previous library")
	}
}

func TestDestinationChangeWaitsForCurrentOperation(t *testing.T) {
	m := scrollModel(t)
	root := m.eng.Root()
	m.run.busy = true
	m = press(m, "esc")
	if m.screen != screenSources || m.eng.Root() != root || !strings.Contains(m.logs[len(m.logs)-1], "cancel it with x") {
		t.Fatal("destination changed during an operation")
	}
}

func TestSpaceIndicatorUpdatesOnlyForCurrentDestination(t *testing.T) {
	m := scrollModel(t)
	m.spaceKnown = false
	if !strings.Contains(m.spaceView(80), "unavailable") {
		t.Fatal("unknown free space was shown as zero")
	}
	next, _ := m.Update(mountsMsg{watched: m.eng.Root(), space: device.SpaceInfo{Available: 0, Total: 1 << 30}, spaceKnown: true})
	m = next.(model)
	if !m.spaceKnown || !strings.Contains(m.spaceView(80), "LOW SPACE") {
		t.Fatal("a full destination was not indicated")
	}
	next, _ = m.Update(mountsMsg{watched: "/old/destination", space: device.SpaceInfo{Available: 1 << 30, Total: 1 << 30}, spaceKnown: true})
	m = next.(model)
	if m.space.Available != 0 {
		t.Fatal("a stale storage update replaced the current destination's capacity")
	}
}
