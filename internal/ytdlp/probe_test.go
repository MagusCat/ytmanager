package ytdlp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requireProbe(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}
}

// Exercise real Opus stream tags and MP3/M4A format tags, including phone paths.
func TestTagsAudioFormats(t *testing.T) {
	requireProbe(t)
	for _, ext := range []string{"opus", "mp3", "m4a"} {
		t.Run(ext, func(t *testing.T) {
			root := t.TempDir()
			local := filepath.Join(root, "ディスコミュ星人."+ext)
			args := []string{"-v", "error", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono", "-t", "0.1",
				"-metadata", "title=ディスコミュ星人", "-metadata", "artist=Artist", "-metadata", "album_artist=Album Artist",
				"-metadata", "album=Album", "-metadata", "track=2/12", local}
			if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
				t.Fatalf("generate audio: %v: %s", err, out)
			}
			phone := filepath.Join(root, "gvfs", "mtp:host=test", "Music", filepath.Base(local))
			if err := CopyFile(local, phone); err != nil {
				t.Fatal(err)
			}
			for _, path := range []string{local, phone} {
				tags, err := Tags(context.Background(), path)
				if err != nil {
					t.Fatal(err)
				}
				for key, want := range map[string]string{"title": "ディスコミュ星人", "artist": "Artist", "album_artist": "Album Artist", "album": "Album"} {
					if tags[key] != want {
						t.Errorf("%s: %s = %q, want %q", path, key, tags[key], want)
					}
				}
				if strings.Split(tags["track"], "/")[0] != "2" {
					t.Errorf("track = %q", tags["track"])
				}
			}
		})
	}
}

func TestTagsErrors(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.opus")
	if _, err := Tags(context.Background(), missing); !errors.Is(err, ErrMissingAudio) || !strings.Contains(err.Error(), "sync (Enter)") {
		t.Fatalf("missing: %v", err)
	}
	empty := filepath.Join(root, "empty.opus")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Tags(context.Background(), empty); !errors.Is(err, ErrInvalidAudio) {
		t.Fatalf("empty: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Tags(ctx, missing); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled: %v", err)
	}
	requireProbe(t)
	corrupt := filepath.Join(root, "Milkshake.opus")
	if err := os.WriteFile(corrupt, []byte("not an opus file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Tags(context.Background(), corrupt); !errors.Is(err, ErrInvalidAudio) || !strings.Contains(err.Error(), "Invalid data found") {
		t.Fatalf("ffprobe diagnostic lost: %v", err)
	}
}

func TestStageProbeCleanup(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "song.opus"), []byte("audio"), 0o644); err != nil {
		t.Fatal(err)
	}
	local, cleanup, err := stageProbe(context.Background(), filepath.Join(root, "song.opus"))
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if b, err := os.ReadFile(local); err != nil || string(b) != "audio" {
		t.Fatalf("staged contents: %q, %v", b, err)
	}
	cleanup()
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatalf("temporary audio not removed: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := stageProbe(ctx, filepath.Join(root, "song.opus")); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled staging: %v", err)
	}
}
