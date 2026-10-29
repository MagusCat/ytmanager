package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ytmanager/internal/manifest"
)

// DeleteTracks removes only explicitly selected managed audio and its lyrics.
// The durable tombstone is saved BEFORE removal: an interrupted operation must
// never cause a later sync to recreate intentionally deleted music.
func (e *Engine) DeleteTracks(ids []string) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return 0, fmt.Errorf("wait for downloads to finish before deleting music")
	}
	root, err := os.OpenRoot(e.m.Root)
	if err != nil {
		return 0, err
	}
	defer root.Close()
	selected := map[string]bool{}
	for _, id := range ids {
		selected[id] = true
	}
	paths := map[string][]string{}
	seen := map[string]bool{}
	for _, id := range ids {
		t := e.m.Tracks[id]
		if t == nil || t.Path == "" {
			continue
		}
		if !filepath.IsLocal(t.Path) || !audioExtension(t.Path) {
			return 0, fmt.Errorf("unsafe audio path: %s", t.Path)
		}
		p := filepath.Clean(t.Path)
		if info, err := root.Lstat(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			return 0, err
		} else if err == nil && !info.Mode().IsRegular() {
			return 0, fmt.Errorf("not a regular audio file: %s", p)
		}
		for other, track := range e.m.Tracks {
			if other != id && !selected[other] && track.Status != manifest.StatusRemoved && filepath.Clean(track.Path) == p {
				return 0, fmt.Errorf("%s shares its file with another track; select both entries", t.Title)
			}
		}
		// Deduplicate repeated input IDs.
		if !seen[id] {
			paths[p] = append(paths[p], id)
			seen[id] = true
		}
	}
	n := 0
	var failures []error
	for p, group := range paths {
		previous := map[string]string{}
		for _, id := range group {
			previous[id] = e.m.Tracks[id].Status
			e.m.Tracks[id].Status = manifest.StatusRemoved
		}
		if err := e.m.Save(); err != nil {
			for id, status := range previous {
				e.m.Tracks[id].Status = status
			}
			failures = append(failures, err)
			break
		}
		if err := root.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
			for id, status := range previous {
				e.m.Tracks[id].Status = status
			}
			failures = append(failures, fmt.Errorf("delete %s: %w", p, err), e.m.Save())
			break
		}
		n += len(group)
		lrc := strings.TrimSuffix(p, filepath.Ext(p)) + ".lrc"
		shared := false
		for _, t := range e.m.Tracks {
			if t.Status != manifest.StatusRemoved && strings.TrimSuffix(filepath.Clean(t.Path), filepath.Ext(t.Path))+".lrc" == lrc {
				shared = true
			}
		}
		if !shared {
			if err := root.Remove(lrc); err != nil && !errors.Is(err, os.ErrNotExist) {
				failures = append(failures, fmt.Errorf("lyrics %s: %w", lrc, err))
			}
		}
	}
	if n > 0 {
		failures = append(failures, e.m.WritePlaylists())
	}
	return n, errors.Join(failures...)
}

func audioExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".opus", ".m4a", ".mp3", ".flac", ".wav", ".ogg", ".aac", ".alac", ".aiff":
		return true
	}
	return false
}
