package engine

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// Exercise real yt-dlp's date filtering, which parses upload_date even when no
// date filter was requested. A four-digit release year here used to crash it.
func TestReleaseYearDoesNotBreakYTDLP(t *testing.T) {
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp not installed")
	}
	for _, year := range []float64{2016, 2025} {
		info := map[string]any{
			"id": "test", "title": "Song", "url": "http://127.0.0.1:1/test.opus",
			"webpage_url": "http://127.0.0.1:1/test", "extractor": "generic", "extractor_key": "Generic",
			"ext": "opus", "upload_date": "20251025", "release_year": year,
		}
		tag(info, "Album", 1, Options{})
		if info["upload_date"] != "20251025" {
			t.Fatal("changed original upload date")
		}
		data, err := json.Marshal(info)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "song.info.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		out, err := exec.CommandContext(ctx, "yt-dlp", "--ignore-config", "--load-info-json", path, "--skip-download", "--print", "filename").CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("release year %.0f rejected by yt-dlp: %v\n%s", year, err, out)
		}
	}
}
