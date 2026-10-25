package tui

import (
	"strings"
	"testing"
	"ytmanager/internal/engine"
)

func TestAlbumActivityFitsAndClears(t *testing.T) {
	m := scrollModel(t)
	m.run.busy = true
	m.run.verb = "downloaded"
	m.apply(engine.Event{Kind: engine.EvStart, ID: "song", Title: "Song", SourceID: "A000", SourceTitle: "A000"})
	m.apply(engine.Event{Kind: engine.EvStart, ID: "other", Title: "Other", SourceID: "A001", SourceTitle: "A001"})
	for _, size := range [][2]int{{80, 24}, {40, 16}, {30, 12}} {
		m = sized(m, size[0], size[1])
		view := assertFits(t, m)
		if !strings.Contains(view, "Downloading: A000") {
			t.Fatalf("album missing at %v:\n%s", size, view)
		}
	}
	m.apply(engine.Event{Kind: engine.EvDone, ID: "song"})
	if strings.Contains(m.activeAlbums(80), "A000") {
		t.Fatal("completed album still shown as active")
	}
	if !strings.Contains(m.activeAlbums(80), "A001") {
		t.Fatal("remaining active album disappeared")
	}
}

func TestParallelAlbumSettingIsSaved(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("AppData", t.TempDir())
	m := scrollModel(t)
	m.screen = screenSettings
	m.cursor = 6
	m.changeSetting(1)
	if m.eng.Options().AlbumJobs != 2 || engine.LoadOptions().AlbumJobs != 2 {
		t.Fatal("parallel album setting not applied and saved")
	}
	m.run.busy = true
	m.changeSetting(1)
	if m.eng.Options().AlbumJobs != 2 {
		t.Fatal("changed worker settings during a download")
	}
}
