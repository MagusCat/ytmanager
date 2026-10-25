package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"ytmanager/internal/manifest"
)

// Unknown failures remain retryable; only a recognized unavailable video is skipped.
func TestRecordFails(t *testing.T) {
	tr := &manifest.Track{}
	for range 6 {
		record(tr, "/", "", "", errors.New("Unable to download webpage: timed out"))
	}
	if tr.Status != manifest.StatusFailed || tr.Fails != 0 {
		t.Fatalf("network errors: status %s fails %d", tr.Status, tr.Fails)
	}
	for i := 1; i <= 6; i++ {
		record(tr, "/", "", "", errors.New("Sign in to confirm your age"))
		if tr.Status != manifest.StatusFailed {
			t.Fatalf("after %d failures status %s", i, tr.Status)
		}
	}
	record(tr, "/r", "/r/a.opus", "", nil)
	if tr.Status != manifest.StatusDone || tr.Fails != 0 || tr.Path != "a.opus" {
		t.Fatalf("success: %+v", tr)
	}
}

// Tracks of one album from different sources must share folder, album artist and year.
func TestTagNormalises(t *testing.T) {
	topic := map[string]any{"uploader": "Bad Bunny - Topic", "artist": "Bad Bunny, Jhay Cortez",
		"artists": []any{"Bad Bunny", "Jhay Cortez"}, "track": "Dákiti", "release_year": 2020.0, "upload_date": "20201030"}
	video := map[string]any{"uploader": "Bad Bunny", "title": "Bad Bunny - Dákiti (Official Video)", "tags": []any{"reggaeton"}}

	if _, title := tag(topic, "El Último Tour Del Mundo", 2, Options{}); title != "Dákiti (feat. Jhay Cortez)" || topic["artist"] != "Bad Bunny" {
		t.Fatalf("feat: %q %+v", title, topic)
	}
	if topic["album_artist"] != "Bad Bunny" || topic["upload_date"] != "20201030" || topic["meta_date"] != "2020" || topic["ytm_prefix"] != "02 - " || topic["genre"] != "" {
		t.Fatalf("topic: %+v", topic)
	}
	if _, title := tag(video, "", 0, Options{}); title != "Dákiti" || video["track"] != "Dákiti" || video["album_artist"] != "Bad Bunny" {
		t.Fatalf("video: %q %+v", title, video)
	}
}

func TestTagOptions(t *testing.T) {
	info := map[string]any{"artist": "A, B", "artists": []any{"A", "B"}, "title": "A - Song (Official Video)"}
	if _, title := tag(info, "", 0, Options{Artists: "all", RawTitles: true}); title != "A - Song (Official Video)" || info["artist"] != "A, B" {
		t.Fatalf("all/raw: %q %+v", title, info)
	}
	info = map[string]any{"artists": []any{"A", "B"}, "track": "Song (feat. B)"}
	if _, title := tag(info, "", 0, Options{}); title != "Song (feat. B)" {
		t.Fatalf("feat added twice: %q", title)
	}
}

func TestRemoveOld(t *testing.T) {
	root := t.TempDir()
	keep := filepath.Join(root, "A", "keep.opus")
	old := filepath.Join(root, "A", "Album", "01 - x.opus")
	os.MkdirAll(filepath.Dir(old), 0o755)
	os.WriteFile(keep, nil, 0o644)
	os.WriteFile(old, nil, 0o644)
	os.WriteFile(filepath.Join(root, "A", "Album", "01 - x.lrc"), nil, 0o644)
	removeOld(root, "A/Album/01 - x.opus")
	if _, err := os.Stat(filepath.Join(root, "A", "Album")); err == nil {
		t.Fatal("empty album folder kept")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("removed a non-empty folder")
	}
}

func TestMoveFile(t *testing.T) {
	root := t.TempDir()
	old := filepath.Join("A, B", "Album", "01 - x.opus")
	os.MkdirAll(filepath.Join(root, "A, B", "Album"), 0o755)
	os.WriteFile(filepath.Join(root, old), []byte("audio"), 0o644)
	os.WriteFile(filepath.Join(root, "A, B", "Album", "01 - x.lrc"), nil, 0o644)
	rel := filepath.Join("A", "Album", "01 - x.opus")
	if err := moveFile(root, old, rel); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(root, rel)); string(b) != "audio" {
		t.Fatal("audio not moved")
	}
	if _, err := os.Stat(filepath.Join(root, "A", "Album", "01 - x.lrc")); err != nil {
		t.Fatal("lyrics not moved")
	}
	if _, err := os.Stat(filepath.Join(root, "A, B")); err == nil {
		t.Fatal("empty old folder kept")
	}
	os.WriteFile(filepath.Join(root, "y.opus"), nil, 0o644)
	if moveFile(root, "y.opus", rel) == nil {
		t.Fatal("overwrote an existing file")
	}
}
