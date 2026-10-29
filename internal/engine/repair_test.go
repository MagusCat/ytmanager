package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"ytmanager/internal/manifest"
)

func TestRepairOpaqueFailuresAndCancellation(t *testing.T) {
	e, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("playlist", "p", "Playlist", "url", []manifest.Entry{{ID: "x", Title: "Song"}})
	e.m.Tracks["x"].Status = manifest.StatusUnavailable
	e.m.Tracks["x"].Error = "exit status 1"
	if err = e.m.Save(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e.Repair(ctx, func(Event) {}) == nil {
		t.Fatal("ignored cancellation")
	}
	if e.m.Tracks["x"].Status != manifest.StatusUnavailable {
		t.Fatal("cancel changed state")
	}
	if err = e.Repair(context.Background(), func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if e.m.Tracks["x"].Status != manifest.StatusPending {
		t.Fatal("did not recover retry")
	}
	backups, _ := filepath.Glob(filepath.Join(e.Root(), "data.music.repair-*"))
	if len(backups) != 1 {
		t.Fatal("missing backup")
	}
	if _, err = os.Stat(filepath.Join(e.Root(), "Playlist.m3u8")); err != nil {
		t.Fatal(err)
	}
}
func TestExportPreservesCustomAndRefreshesOwned(t *testing.T) {
	e, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("playlist", "p", "Playlist", "url", []manifest.Entry{{ID: "x", Title: "Song"}})
	if err = e.ExportM3U(context.Background(), func(Event) {}); err != nil {
		t.Fatal(err)
	}
	e.m.Tracks["x"].Status = manifest.StatusDone
	e.m.Tracks["x"].Path = "song.opus"
	if err = e.ExportM3U(context.Background(), func(Event) {}); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(e.Root(), "Playlist.m3u")
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "song.opus") {
		t.Fatal("export not refreshed")
	}
	os.WriteFile(p, []byte("custom"), 0600)
	if err = e.ExportM3U(context.Background(), func(Event) {}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(p)
	if string(b) != "custom" {
		t.Fatal("custom overwritten")
	}
}
