package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ytmanager/internal/manifest"
)

func libraryFixture(t *testing.T) *Engine {
	t.Helper()
	e, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("album", "album", "Album", "https://music.youtube.com/playlist?list=album", []manifest.Entry{{ID: "a", Title: "Song", Duration: 180}, {ID: "b", Title: "SONG!", Duration: 185}, {ID: "c", Title: "Other", Duration: 200}})
	e.m.Sources[0].Creator = "Artist"
	for id, tr := range e.m.Tracks {
		tr.Path = id + ".opus"
		tr.Status = manifest.StatusDone
		if err := os.WriteFile(filepath.Join(e.Root(), tr.Path), []byte("audio"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.m.Save(); err != nil {
		t.Fatal(err)
	}
	return e
}

func TestLibrarySearchAndCandidates(t *testing.T) {
	e := libraryFixture(t)
	if got := FilterLibrary(e.Library(), "artist album song", true); len(got) != 2 {
		t.Fatalf("matches: %+v", got)
	}
	if got := FilterLibrary(e.Library(), "c.opus", false); len(got) != 1 || got[0].ID != "c" {
		t.Fatalf("path search: %+v", got)
	}
	if got := FilterLibrary(e.Library(), "other", true); len(got) != 0 {
		t.Fatal(got)
	}
}

func TestDeleteTracksPersistsAndSkipsSync(t *testing.T) {
	e := libraryFixture(t)
	if err := os.WriteFile(filepath.Join(e.Root(), "a.lrc"), []byte("lyrics"), 0600); err != nil {
		t.Fatal(err)
	}
	n, err := e.DeleteTracks([]string{"a", "a"})
	if err != nil || n != 1 {
		t.Fatalf("%d %v", n, err)
	}
	for _, p := range []string{"a.opus", "a.lrc"} {
		if _, err := os.Stat(filepath.Join(e.Root(), p)); !os.IsNotExist(err) {
			t.Fatalf("still present: %s", p)
		}
	}
	reopened, err := Open(e.Root(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if reopened.HasPending() {
		t.Fatal("intentional deletion became pending")
	}
	reopened.m.Merge("album", "album", "Album", "url", []manifest.Entry{{ID: "a", Title: "Song"}, {ID: "b", Title: "SONG!"}, {ID: "c", Title: "Other"}})
	if got := reopened.m.Pending([]string{"album"}); len(got) > 0 {
		t.Fatalf("pending after refresh: %v", got)
	}
	if len(reopened.Library()) != 2 {
		t.Fatal("deleted track visible")
	}
	playlist, err := os.ReadFile(filepath.Join(e.Root(), "Album.m3u8"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(playlist), "a.opus") || !strings.Contains(string(playlist), "b.opus") {
		t.Fatal(string(playlist))
	}
}

func TestDeleteRejectsEscapesAndSharedPaths(t *testing.T) {
	for _, path := range []string{"../outside.opus", "data.music", "linked/outside.opus"} {
		t.Run(path, func(t *testing.T) {
			e := libraryFixture(t)
			outside := t.TempDir()
			file := filepath.Join(outside, "outside.opus")
			if err := os.WriteFile(file, []byte("keep"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(e.Root(), "linked")); err != nil {
				t.Fatal(err)
			}
			e.m.Tracks["a"].Path = path
			if n, err := e.DeleteTracks([]string{"a"}); err == nil || n != 0 {
				t.Fatalf("accepted %q: %d %v", path, n, err)
			}
			if _, err := os.Stat(file); err != nil {
				t.Fatal("outside file removed", err)
			}
		})
	}
	e := libraryFixture(t)
	e.m.Tracks["b"].Path = "a.opus"
	if _, err := e.DeleteTracks([]string{"a"}); err == nil {
		t.Fatal("deleted shared unselected file")
	}
	if n, err := e.DeleteTracks([]string{"a", "b"}); err != nil || n != 2 {
		t.Fatalf("shared selection: %d %v", n, err)
	}
}

func TestDeleteSaveFailurePreservesAudio(t *testing.T) {
	e := libraryFixture(t)
	if err := os.Mkdir(filepath.Join(e.Root(), "data.music.tmp"), 0700); err != nil {
		t.Fatal(err)
	}
	if n, err := e.DeleteTracks([]string{"a"}); err == nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
	if _, err := os.Stat(filepath.Join(e.Root(), "a.opus")); err != nil {
		t.Fatal(err)
	}
	if e.m.Tracks["a"].Status != manifest.StatusDone {
		t.Fatal("state not restored")
	}
}

func TestInterruptedDeleteRemainsVisibleAndRetryable(t *testing.T) {
	e := libraryFixture(t)
	e.m.Tracks["a"].Status = manifest.StatusRemoved
	if len(e.Library()) != 3 {
		t.Fatal("stranded audio hidden")
	}
	if n, err := e.DeleteTracks([]string{"a"}); err != nil || n != 1 {
		t.Fatalf("retry: %d %v", n, err)
	}
	if len(e.Library()) != 2 {
		t.Fatal("deleted audio still visible")
	}
}

func TestDeleteRefusesActiveDownloads(t *testing.T) {
	e := libraryFixture(t)
	e.running = true
	if n, err := e.DeleteTracks([]string{"a"}); err == nil || n != 0 {
		t.Fatalf("%d %v", n, err)
	}
}
