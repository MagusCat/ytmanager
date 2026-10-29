package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"ytmanager/internal/engine"
	"ytmanager/internal/manifest"
)

func libraryModel(t *testing.T) model {
	t.Helper()
	root := t.TempDir()
	data, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	data.Merge("album", "album", "Album", "url", []manifest.Entry{{ID: "a", Title: "Song"}, {ID: "b", Title: "Song"}, {ID: "c", Title: "Other"}})
	for id, tr := range data.Tracks {
		tr.Path = id + ".opus"
		tr.Status = manifest.StatusDone
		if err := os.WriteFile(filepath.Join(root, tr.Path), []byte("audio"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := data.Save(); err != nil {
		t.Fatal(err)
	}
	m := newModel()
	if err := m.openDest(root); err != nil {
		t.Fatal(err)
	}
	return press(m, "l")
}

func TestLibrarySearchCancelAndFilteredDeletion(t *testing.T) {
	m := libraryModel(t)
	m = press(m, "a", "/", "Song", "enter", "d")
	if len(m.library.confirm) != 2 {
		t.Fatalf("hidden marks included: %v", m.library.confirm)
	}
	m = press(m, "esc")
	if len(m.eng.Library()) != 3 {
		t.Fatal("cancel removed files")
	}
	m = press(m, "/", "esc")
	if m.library.query != "Song" {
		t.Fatal("cancel lost prior search")
	}
	m = press(m, "d")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if !m.library.busy || cmd == nil {
		t.Fatal("no async deletion")
	}
	next, _ = m.Update(cmd())
	m = next.(model)
	if m.library.busy || len(m.eng.Library()) != 1 {
		t.Fatal("delete did not complete")
	}
	if m.eng.Library()[0].Title != "Other" {
		t.Fatal("deleted hidden selection")
	}
}

func TestLibraryDuplicateFilterAndBounds(t *testing.T) {
	m := libraryModel(t)
	m = press(m, "f")
	if len(m.libraryRows()) != 2 {
		t.Fatal("duplicate filter")
	}
	for _, size := range [][2]int{{30, 12}, {48, 18}, {80, 24}, {120, 40}} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			m.width, m.height = size[0], size[1]
			for _, state := range []model{m, press(m, "/"), press(m, "d")} {
				view := state.View()
				lines := strings.Split(view, "\n")
				if len(lines) > m.height {
					t.Fatal("too tall")
				}
				for _, line := range lines {
					if ansi.StringWidth(line) > m.width {
						t.Fatal("too wide")
					}
				}
				if state.library.confirm != nil && !strings.Contains(ansi.Strip(view), "esc cancel") {
					t.Fatal("missing cancellation", view)
				}
			}
		})
	}
}

func TestLibraryBusyEntryIsBlocked(t *testing.T) {
	m := libraryModel(t)
	m.screen = screenSources
	m.run.busy = true
	m = press(m, "l")
	if m.screen != screenSources {
		t.Fatal("opened during downloads")
	}
}

func TestLibraryConfirmationScrollsWithLogsAtMinimumSize(t *testing.T) {
	m := libraryModel(t)
	m.width, m.height = 30, 12
	for i := 0; i < 50; i++ {
		m.library.all = append(m.library.all, engine.LibraryTrack{ID: fmt.Sprint(i), Title: fmt.Sprintf("Track %02d", i), Path: fmt.Sprintf("%02d.opus", i)})
	}
	m.log("Earlier download message")
	m = press(m, "a", "d")
	for _, key := range []string{"down", "end", "home"} {
		m = press(m, key)
		view := ansi.Strip(m.View())
		target := m.library.confirm[m.cursor]
		var row engine.LibraryTrack
		for _, r := range m.library.all {
			if r.ID == target {
				row = r
			}
		}
		for _, want := range []string{row.Title, row.Path, "No undo.", "Skipped by future syncs.", "esc cancel"} {
			if !strings.Contains(view, want) {
				t.Fatalf("missing %q at %s:\n%s", want, key, view)
			}
		}
	}
	if len(m.logs) != 1 {
		t.Fatal("confirmation discarded logs")
	}
}
