package engine

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"ytmanager/internal/manifest"
)

func TestRearrangeDownloadedAudio(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe", "yt-dlp"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
	for _, phone := range []bool{false, true} {
		name := "folder"
		if phone {
			name = "phone"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			if phone {
				root = filepath.Join(root, "gvfs", "mtp:host=test", "Music")
			}
			e, err := Open(root, Options{Jobs: 3})
			if err != nil {
				t.Fatal(err)
			}
			source := e.m.Merge("playlist", "playlist", "Library", "unused", []manifest.Entry{
				{ID: "tagged", Title: "ディスコミュ星人"}, {ID: "untitled", Title: "Milkshake"},
				{ID: "missing", Title: "Missing"}, {ID: "empty", Title: "Empty"}, {ID: "corrupt", Title: "Corrupt"},
			})
			original := map[string][]byte{}
			for _, id := range source.Tracks {
				track := e.m.Tracks[id]
				track.Path, track.Status = id+".opus", manifest.StatusDone
				path := filepath.Join(root, track.Path)
				switch id {
				case "tagged", "untitled":
					args := []string{"-v", "error", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono", "-t", "0.1",
						"-metadata", "artist=Artist", "-metadata", "album=Album", "-metadata", "track=2/12"}
					if id == "tagged" {
						args = append(args, "-metadata", "title="+track.Title)
					}
					if out, err := exec.Command("ffmpeg", append(args, path)...).CombinedOutput(); err != nil {
						t.Fatalf("generate audio: %v: %s", err, out)
					}
					if err := os.WriteFile(filepath.Join(root, id+".lrc"), []byte("lyrics"), 0o644); err != nil {
						t.Fatal(err)
					}
				case "empty":
					if err := os.WriteFile(path, nil, 0o644); err != nil {
						t.Fatal(err)
					}
				case "corrupt":
					if err := os.WriteFile(path, []byte("not an opus file"), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				original[id], _ = os.ReadFile(path)
			}
			if err := e.m.Save(); err != nil {
				t.Fatal(err)
			}
			var mu sync.Mutex
			var failures []Event
			err = e.Rearrange(context.Background(), func(event Event) {
				mu.Lock()
				defer mu.Unlock()
				if event.Kind == EvFailed {
					failures = append(failures, event)
				}
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(failures) != 3 {
				t.Fatalf("failures = %+v", failures)
			}
			for _, event := range failures {
				if !strings.Contains(event.Msg, "sync (Enter)") {
					t.Errorf("no recovery action: %s", event.Msg)
				}
			}
			persisted, err := manifest.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, id := range source.Tracks {
				track := persisted.Tracks[id]
				if id == "tagged" || id == "untitled" {
					want := filepath.Join("Artist", "Album", "02 - "+track.Title+".opus")
					if track.Path != want || track.Status != manifest.StatusDone {
						t.Errorf("moved %s: %+v", id, track)
					}
					if b, err := os.ReadFile(filepath.Join(root, want)); err != nil || string(b) != string(original[id]) {
						t.Errorf("audio changed: %s: %v", id, err)
					}
					lrc := strings.TrimSuffix(want, ".opus") + ".lrc"
					if b, err := os.ReadFile(filepath.Join(root, lrc)); err != nil || string(b) != "lyrics" {
						t.Errorf("lyrics lost: %s: %v", id, err)
					}
				} else {
					if track.Status != manifest.StatusPending || track.Error == "" {
						t.Errorf("repair not persisted: %s: %+v", id, track)
					}
					if id != "missing" {
						if b, err := os.ReadFile(filepath.Join(root, track.Path)); err != nil || string(b) != string(original[id]) {
							t.Errorf("failed file modified: %s: %v", id, err)
						}
					}
				}
			}
			if pending := persisted.Pending([]string{source.ID}); len(pending) != 3 {
				t.Errorf("sync should restore only three damaged/missing tracks: %v", pending)
			}
		})
	}
}

func TestRearrangeMissingTools(t *testing.T) {
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Tracks["song"] = &manifest.Track{Title: "Song", Path: "song.opus", Status: manifest.StatusDone}
	t.Setenv("PATH", t.TempDir())
	err = e.Rearrange(context.Background(), func(Event) { t.Error("work started without ffprobe") })
	if err == nil || !strings.Contains(err.Error(), "ffprobe is missing") {
		t.Fatalf("missing tool: %v", err)
	}
	if e.m.Tracks["song"].Status != manifest.StatusDone {
		t.Fatal("missing tools should not queue downloads")
	}
}

func TestMoveDisconnectedDestination(t *testing.T) {
	root := filepath.Join(t.TempDir(), "disconnected")
	e, err := Open(root, Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	track := &manifest.Track{Title: "Song", Path: "song.opus", Status: manifest.StatusDone}
	e.m.Tracks["song"] = track
	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	var failure Event
	e.move(context.Background(), "song", func(event Event) {
		if event.Kind == EvFailed {
			failure = event
		}
	})
	if !strings.Contains(failure.Msg, "reconnect") || track.Status != manifest.StatusDone {
		t.Fatalf("disconnect queued a download: %+v, %+v", failure, track)
	}
}
