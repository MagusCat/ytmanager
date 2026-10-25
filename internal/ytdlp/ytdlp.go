// Package ytdlp wraps the yt-dlp binary: list sources, read metadata and download audio.
package ytdlp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Entry struct {
	creatorMetadata
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Duration    float64 `json:"duration"`
	Unavailable bool    `json:"-"` // listed as private or deleted
}

type Listing struct {
	Type, ID, Title, Creator string
	Entries                  []Entry
}

// Kind classifies a URL. YT Music albums are OLAK5uy_ playlists or browse/MPREb pages; RD lists
// are generated mixes that change on every read, except RDCLAK, which are curated playlists.
func Kind(url string) string {
	switch {
	case strings.Contains(url, "list=OLAK5uy_"), strings.Contains(url, "/browse/MPREb"):
		return "album"
	case strings.Contains(url, "list=RD") && !strings.Contains(url, "list=RDCLAK"):
		return "mix"
	case strings.Contains(url, "list="):
		return "playlist"
	}
	return "track"
}

func base(cookies string) []string {
	a := []string{"--no-warnings", "--color", "never"}
	if cookies != "" {
		a = append(a, "--cookies-from-browser", Keyring(cookies, runtime.GOOS, os.Getenv("XDG_CURRENT_DESKTOP")))
	}
	return a
}

func output(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "yt-dlp", args...).Output()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return nil, errors.New(lastErr(string(ee.Stderr)))
	}
	return out, err
}

func lastErr(stderr string) string {
	lines := strings.Split(strings.TrimSpace(stderr), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.HasPrefix(lines[i], "ERROR:") {
			return strings.TrimPrefix(lines[i], "ERROR: ")
		}
	}
	return lines[len(lines)-1]
}

func List(ctx context.Context, url, cookies string) (*Listing, error) {
	out, err := output(ctx, append(base(cookies), "--flat-playlist", "-J", url)...)
	if err != nil {
		return nil, err
	}
	return parseListing(out, url)
}

func parseListing(out []byte, url string) (*Listing, error) {
	var r struct {
		creatorMetadata
		Type    string  `json:"_type"`
		ID      string  `json:"id"`
		Title   string  `json:"title"`
		Dur     float64 `json:"duration"`
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, err
	}
	l := &Listing{Type: Kind(url), ID: r.ID, Title: strings.TrimPrefix(r.Title, "Album - ")}
	if r.Type != "playlist" {
		l.Type = "track"
		l.Creator = r.creatorMetadata.creator()
		l.Entries = []Entry{{ID: r.ID, Title: r.Title, Duration: r.Dur}}
		return l, nil
	}
	for _, e := range r.Entries {
		if e.ID == "" {
			continue
		}
		e.Unavailable = e.Title == "[Private video]" || e.Title == "[Deleted video]"
		l.Entries = append(l.Entries, e)
	}
	l.Creator = listingCreator(l.Type, r.creatorMetadata, l.Entries)
	return l, nil
}

// Info returns the full info-json of a track so it can be enriched before downloading.
func Info(ctx context.Context, id, cookies string) (map[string]any, error) {
	out, err := output(ctx, append(base(cookies), "-J", "--no-playlist", "https://www.youtube.com/watch?v="+id)...)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	return m, json.Unmarshal(out, &m)
}
