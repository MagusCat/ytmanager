// Package manifest reads and writes data.music: which sources (playlists, albums, mixes) live in a
// destination and the state of every track, so syncing never downloads twice.
package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const FileName = "data.music"

const (
	StatusPending     = "pending"
	StatusDone        = "done"
	StatusRemoved     = "removed"     // intentionally deleted; never automatically downloaded again
	StatusFailed      = "failed"      // retried on the next sync
	StatusUnavailable = "unavailable" // skipped until it gets an alternative
)

type Source struct {
	Type     string    `json:"type"` // playlist | album | mix | track
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Creator  string    `json:"creator,omitempty"` // artist, channel or playlist owner
	URL      string    `json:"url"`
	LastSync time.Time `json:"last_sync"`
	Tracks   []string  `json:"tracks"`
}

type Track struct {
	Title    string  `json:"title"`
	Duration float64 `json:"duration,omitempty"`
	Path     string  `json:"path,omitempty"` // relative to the destination root
	Status   string  `json:"status"`
	Alt      string  `json:"alt,omitempty"`    // YouTube id downloaded instead of this one, in its slot
	Lyrics   string  `json:"lyrics,omitempty"` // synced | plain
	Error    string  `json:"error,omitempty"`
	Fails    int     `json:"fails,omitempty"` // consecutive non-network failures
}

type Manifest struct {
	Version int               `json:"version"`
	Format  string            `json:"format"`
	Sources []*Source         `json:"sources"`
	Tracks  map[string]*Track `json:"tracks"`
	Queue   []SourceRequest   `json:"queue,omitempty"`
	Root    string            `json:"-"`
}

// SourceRequest survives closing the app before a source has been listed.
type SourceRequest struct {
	URL     string `json:"url"`
	Refresh bool   `json:"refresh,omitempty"`
}

type Entry struct {
	ID, Title   string
	Duration    float64
	Unavailable bool
}

func (m *Manifest) Source(id string) *Source {
	for _, s := range m.Sources {
		if s.ID == id {
			return s
		}
	}
	return nil
}

// Merge creates or updates a source; new tracks start pending and downloaded ones are kept.
// Mixes change on every read, so their tracks accumulate instead of being replaced.
func (m *Manifest) Merge(typ, id, title, url string, entries []Entry) *Source {
	s := m.Source(id)
	if s == nil {
		s = &Source{Type: typ, ID: id}
		m.Sources = append(m.Sources, s)
	}
	var keep []string
	if typ == "mix" {
		keep = s.Tracks
	}
	s.Title, s.URL, s.LastSync, s.Tracks = title, url, time.Now().UTC(), keep
	have := map[string]bool{}
	for _, tid := range keep {
		have[tid] = true
	}
	for _, e := range entries {
		if !have[e.ID] {
			have[e.ID] = true
			s.Tracks = append(s.Tracks, e.ID)
		}
		if m.Tracks[e.ID] == nil {
			t := &Track{Title: e.Title, Duration: e.Duration, Status: StatusPending}
			if e.Unavailable {
				t.Status = StatusUnavailable
			}
			m.Tracks[e.ID] = t
		}
	}
	return s
}

// Remove drops the source from the manifest; its audio stays because another source may share it.
func (m *Manifest) Remove(id string) {
	for i, s := range m.Sources {
		if s.ID == id {
			m.Sources = append(m.Sources[:i], m.Sources[i+1:]...)
			return
		}
	}
}

// Pending returns the unique tracks of those sources that are missing, failed or whose file is gone.
func (m *Manifest) Pending(sourceIDs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, sid := range sourceIDs {
		s := m.Source(sid)
		if s == nil {
			continue
		}
		for _, id := range s.Tracks {
			if seen[id] {
				continue
			}
			seen[id] = true
			t := m.Tracks[id]
			if t.Status == StatusUnavailable || t.Status == StatusRemoved {
				continue
			}
			if t.Status == StatusDone {
				if actual := existingAudioPath(m.Root, t.Path); actual != "" {
					t.Path = actual
					continue
				}
				t.Status = StatusPending
			}
			out = append(out, id)
		}
	}
	return out
}

// AlbumOf returns the album title and track number if the track belongs to a saved album.
func (m *Manifest) AlbumOf(trackID string) (string, int) {
	for _, s := range m.Sources {
		if s.Type != "album" {
			continue
		}
		for i, id := range s.Tracks {
			if id == trackID {
				return s.Title, i + 1
			}
		}
	}
	return "", 0
}

func (m *Manifest) Count(s *Source) (done, total, unavailable int) {
	for _, id := range s.Tracks {
		switch m.Tracks[id].Status {
		case StatusDone:
			done++
		case StatusUnavailable:
			unavailable++
		}
	}
	return done, len(s.Tracks), unavailable
}

// existingAudioPath resolves unique case-only variants returned by MTP.
func existingAudioPath(root, rel string) string {
	if rel == "" || filepath.IsAbs(rel) {
		return ""
	}
	current := root
	for _, part := range strings.Split(filepath.Clean(rel), string(filepath.Separator)) {
		if part == ".." {
			return ""
		}
		entries, err := os.ReadDir(current)
		if err != nil {
			return ""
		}
		match := ""
		for _, entry := range entries {
			if strings.EqualFold(part, entry.Name()) {
				if match != "" {
					return ""
				}
				match = entry.Name()
			}
		}
		if match == "" {
			return ""
		}
		current = filepath.Join(current, match)
	}
	stat, err := os.Stat(current)
	if err != nil || !stat.Mode().IsRegular() || stat.Size() == 0 {
		return ""
	}
	actual, _ := filepath.Rel(root, current)
	return actual
}
