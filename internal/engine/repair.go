package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

// Repair reconciles existing audio by its embedded source ID and rebuilds playlists.
// It never deletes or rewrites audio. Ambiguous paths are left for manual review.
func (e *Engine) Repair(ctx context.Context, emit func(Event)) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return fmt.Errorf("wait for downloads to finish before repairing")
	}
	data, err := json.Marshal(e.m)
	if err != nil {
		e.mu.Unlock()
		return err
	}
	m := &manifest.Manifest{Root: e.m.Root}
	if err = json.Unmarshal(data, m); err != nil {
		e.mu.Unlock()
		return err
	}
	e.running = true
	e.mu.Unlock()
	defer func() { e.mu.Lock(); e.running = false; e.mu.Unlock() }()
	emit(Event{Kind: EvLog, Msg: "Checking library paths and embedded song IDs…"})
	index := map[string][]string{}
	err = filepath.WalkDir(m.Root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		rel, _ := filepath.Rel(m.Root, p)
		index[strings.ToLower(rel)] = append(index[strings.ToLower(rel)], rel)
		return nil
	})
	if err != nil {
		return err
	}
	backup := filepath.Join(m.Root, "data.music.repair-"+time.Now().UTC().Format("20060102T150405.000000000"))
	original, err := os.ReadFile(filepath.Join(m.Root, manifest.FileName))
	if err != nil {
		return err
	}
	if err = os.WriteFile(backup, original, 0600); err != nil {
		return err
	}
	repaired := 0
	for id, t := range m.Tracks {
		if t.Status == manifest.StatusRemoved {
			continue
		}
		candidates := index[strings.ToLower(t.Path)]
		if t.Path != "" && len(candidates) == 1 && (candidates[0] != t.Path || t.Status != manifest.StatusDone) {
			rel := candidates[0]
			tags, err := ytdlp.Tags(ctx, filepath.Join(m.Root, rel))
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				continue
			}
			u, _ := url.Parse(tags["purl"])
			want := id
			if t.Alt != "" {
				want = t.Alt
			}
			if u == nil || u.Query().Get("v") != want {
				continue
			}
			t.Path = rel
			t.Status = manifest.StatusDone
			t.Error = ""
			t.Fails = 0
			repaired++
			emit(Event{Kind: EvLog, Msg: "Recovered: " + t.Title})
		}
		if t.Status == manifest.StatusUnavailable && t.Error == "exit status 1" {
			t.Status = manifest.StatusPending
			t.Error = ""
			t.Fails = 0
		}
	}
	if err = m.Save(); err != nil {
		return err
	}
	e.mu.Lock()
	e.m = m
	e.mu.Unlock()
	if err = m.WritePlaylists(); err != nil {
		return err
	}
	emit(Event{Kind: EvLog, Msg: fmt.Sprintf("Reconciled %d files. Playlists rebuilt. Backup: %s", repaired, backup)})
	return nil
}
