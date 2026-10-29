package engine

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"ytmanager/internal/manifest"
	"ytmanager/internal/ytmusic"
)

func TestUnconfirmedSongDoesNotDownloadOrRemoveOldFile(t *testing.T) {
	root := t.TempDir()
	e, err := Open(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("playlist", "list", "List", "url", []manifest.Entry{{ID: "video", Title: "Song"}})
	tr := e.m.Tracks["video"]
	tr.Path = "existing.opus"
	if err := os.WriteFile(filepath.Join(root, tr.Path), []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	// No external binaries: reaching the download path would produce a different error.
	t.Setenv("PATH", t.TempDir())
	e.resolveSong = func(ctx context.Context, id string) (string, error) {
		if id != "video" {
			t.Fatalf("unexpected ID %q", id)
		}
		return "", ytmusic.ErrNoSong
	}
	var failed bool
	e.track(context.Background(), "video", func(ev Event) { failed = failed || ev.Kind == EvFailed })
	if !failed || tr.Status != manifest.StatusFailed || tr.Error != ytmusic.ErrNoSong.Error() {
		t.Fatalf("unexpected result: %+v", tr)
	}
	if len(e.Missing()) != 1 {
		t.Fatal("song is not available in alternative selection")
	}
	if data, err := os.ReadFile(filepath.Join(root, tr.Path)); err != nil || string(data) != "original" {
		t.Fatalf("old file changed: %s %v", data, err)
	}
	if err := e.Retry("video"); err != nil {
		t.Fatal(err)
	}
	if tr.Status != manifest.StatusPending {
		t.Fatal("cannot retry")
	}
}

func TestExplicitAlternativeBypassesSongResolution(t *testing.T) {
	e, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("track", "video", "Song", "url", []manifest.Entry{{ID: "video", Title: "Song"}})
	e.m.Tracks["video"].Alt = "chosen"
	e.resolveSong = func(context.Context, string) (string, error) {
		t.Fatal("explicit alternative was replaced")
		return "", nil
	}
	t.Setenv("PATH", t.TempDir())
	e.track(context.Background(), "video", func(Event) {})
	if e.m.Tracks["video"].Status != manifest.StatusFailed {
		t.Fatal("expected missing yt-dlp")
	}
}

func TestResolvedSongIsPassedToDownloader(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX test executable")
	}
	e, err := Open(t.TempDir(), Options{})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("track", "video", "Song", "url", []manifest.Entry{{ID: "video", Title: "Song"}})
	e.resolveSong = func(context.Context, string) (string, error) { return "audio", nil }
	bin := t.TempDir()
	// Stop at metadata lookup, before any audio or lyrics network request.
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte("#!/bin/sh\nprintf 'ERROR: requested %s\\n' \"$*\" >&2\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	e.track(context.Background(), "video", func(Event) {})
	tr := e.m.Tracks["video"]
	if !strings.Contains(tr.Error, "watch?v=audio") || strings.Contains(tr.Error, "watch?v=video") {
		t.Fatalf("wrong recording requested: %s", tr.Error)
	}
	if _, ok := e.m.Tracks["audio"]; ok {
		t.Fatal("source identity was replaced")
	}
}
