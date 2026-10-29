package engine

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"ytmanager/internal/manifest"
)

// LibraryTrack describes a managed file. Sources and creators are searchable context,
// not a claim that a playlist owner is the recording artist.
type LibraryTrack struct {
	ID, Title, Path, Sources, Creators string
	Duration                           float64
	Duplicate                          bool
	Unmanaged                          bool
	Size                               int64
	Note                               string
}

func (e *Engine) Library() []LibraryTrack {
	e.mu.Lock()
	defer e.mu.Unlock()
	contexts := map[string]struct{ sources, creators []string }{}
	for _, source := range e.m.Sources {
		for _, id := range source.Tracks {
			c := contexts[id]
			c.sources = append(c.sources, source.Title)
			if source.Creator != "" {
				c.creators = append(c.creators, source.Creator)
			}
			contexts[id] = c
		}
	}
	var out []LibraryTrack
	for id, t := range e.m.Tracks {
		if t.Path == "" {
			continue
		}
		// A crash after saving the tombstone may leave the file behind.
		// Keep that file visible so the user can retry the deletion.
		if t.Status == manifest.StatusRemoved {
			if _, err := os.Lstat(filepath.Join(e.m.Root, t.Path)); os.IsNotExist(err) {
				continue
			}
		}
		row := LibraryTrack{ID: id, Title: t.Title, Path: t.Path, Duration: t.Duration}
		row.Sources = strings.Join(contexts[id].sources, " · ")
		row.Creators = strings.Join(contexts[id].creators, " · ")
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Title), strings.ToLower(out[j].Title)
		if a != b {
			return a < b
		}
		return out[i].ID < out[j].ID
	})
	// Similar titles flag candidates only. Different editions or artists can match;
	// the UI explicitly asks the user to compare them before deleting anything.
	groups := map[string][]int{}
	for i, t := range out {
		key := strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				return unicode.ToLower(r)
			}
			return -1
		}, t.Title)
		if key != "" {
			groups[key] = append(groups[key], i)
		}
	}
	for _, group := range groups {
		if len(group) > 1 {
			for _, i := range group {
				out[i].Duplicate = true
			}
		}
	}
	return out
}

func FilterLibrary(all []LibraryTrack, query string, duplicates bool) []LibraryTrack {
	terms := strings.Fields(strings.ToLower(query))
	var out []LibraryTrack
	for _, t := range all {
		if duplicates && !t.Duplicate {
			continue
		}
		hay := strings.ToLower(strings.Join([]string{t.Title, t.Path, t.Sources, t.Creators, t.ID}, " "))
		match := true
		for _, term := range terms {
			if !strings.Contains(hay, term) {
				match = false
				break
			}
		}
		if match {
			out = append(out, t)
		}
	}
	return out
}
