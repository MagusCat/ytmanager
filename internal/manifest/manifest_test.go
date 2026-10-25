package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMergePendingSave(t *testing.T) {
	root := t.TempDir()
	m, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	m.Merge("playlist", "PL1", "Gym: 2026?", "u", []Entry{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}})
	m.Merge("album", "OLAK1", "Disco", "u2", []Entry{{ID: "b"}, {ID: "c"}})

	if got := m.Pending([]string{"PL1", "OLAK1"}); len(got) != 3 {
		t.Fatalf("dedup: want 3 unique tracks, got %v", got)
	}
	if album, n := m.AlbumOf("c"); album != "Disco" || n != 2 {
		t.Fatalf("AlbumOf = %q %d", album, n)
	}

	os.WriteFile(filepath.Join(root, "a.opus"), nil, 0o644)
	m.Tracks["a"].Status, m.Tracks["a"].Path = "done", "a.opus"
	m.Tracks["b"].Status, m.Tracks["b"].Path = "done", "borrado.opus" // missing file => downloaded again
	if got := m.Pending([]string{"PL1"}); len(got) != 1 || got[0] != "b" {
		t.Fatalf("Pending = %v", got)
	}

	// re-syncing the playlist with a new track keeps the state of the old ones
	m.Merge("playlist", "PL1", "Gym: 2026?", "u", []Entry{{ID: "a"}, {ID: "d"}})
	if m.Tracks["a"].Status != "done" || m.Tracks["d"].Status != "pending" {
		t.Fatal("merge overwrote the state")
	}

	if err := m.Save(); err != nil {
		t.Fatal(err)
	}
	if err := m.WritePlaylists(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "Gym_ 2026_.m3u8")); err != nil {
		t.Fatal("m3u8 with safe name not created:", err)
	}
	m2, err := Load(root)
	if err != nil || len(m2.Sources) != 2 || m2.Tracks["a"].Path != "a.opus" {
		t.Fatalf("roundtrip failed: %v", err)
	}
}

// An unavailable track is neither retried nor counted as missing; with an alternative it is pending again.
func TestUnavailableSkipped(t *testing.T) {
	m, _ := Load(t.TempDir())
	s := m.Merge("playlist", "PL", "Mix", "u", []Entry{{ID: "a"}, {ID: "b"}})
	m.Tracks["a"].Status = "unavailable"
	if got := m.Pending([]string{"PL"}); len(got) != 1 || got[0] != "b" {
		t.Fatalf("Pending = %v", got)
	}
	if d, total, u := m.Count(s); d != 0 || total != 2 || u != 1 {
		t.Fatalf("Count = %d %d %d", d, total, u)
	}
	m.Tracks["a"].Status, m.Tracks["a"].Alt = "pending", "zzz"
	if got := m.Pending([]string{"PL"}); len(got) != 2 {
		t.Fatalf("with alternative Pending = %v", got)
	}
}

// A mix keeps the tracks it had while adding new ones; private entries start unavailable.
func TestMergeMix(t *testing.T) {
	m, _ := Load(t.TempDir())
	m.Merge("mix", "RD", "Supermix", "u", []Entry{{ID: "a"}, {ID: "b"}})
	s := m.Merge("mix", "RD", "Supermix", "u", []Entry{{ID: "b"}, {ID: "c"}, {ID: "p", Unavailable: true}})
	if got := strings.Join(s.Tracks, ","); got != "a,b,c,p" {
		t.Fatalf("mix tracks = %s", got)
	}
	if m.Tracks["p"].Status != StatusUnavailable {
		t.Fatal("private entry should start unavailable")
	}
}

// If the device is unplugged while data.music is being written, Load falls back to the backup.
func TestLoadFallsBackToBackup(t *testing.T) {
	root := t.TempDir()
	m, _ := Load(root)
	m.Format = "mp3"
	b, _ := json.Marshal(m)
	os.WriteFile(filepath.Join(root, FileName+".bak"), b, 0o644)
	os.WriteFile(filepath.Join(root, FileName), b[:len(b)/2], 0o644) // cut short
	got, err := Load(root)
	if err != nil || got.Format != "mp3" {
		t.Fatalf("Load = %+v, %v", got, err)
	}
}
