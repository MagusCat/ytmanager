package engine

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"ytmanager/internal/ytdlp"
)

// ScanLibrary inventories actual audio under the selected destination, including
// unmanaged files. Hash matches are byte-identical; title matches are candidates.
func (e *Engine) ScanLibrary(ctx context.Context) ([]LibraryTrack, error) {
	managed := e.Library()
	byPath := map[string]LibraryTrack{}
	for _, t := range managed {
		byPath[t.Path] = t
	}
	var rows []LibraryTrack
	hashes := map[string][]int{}
	titles := map[string][]int{}
	err := filepath.WalkDir(e.Root(), func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || !audioExtension(p) {
			return nil
		}
		rel, _ := filepath.Rel(e.Root(), p)
		stat, err := d.Info()
		if err != nil {
			return err
		}
		row, known := byPath[rel]
		if !known {
			row = LibraryTrack{ID: "file:" + rel, Title: strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())), Sources: "Unmanaged file"}
		}
		row.Path = rel
		row.Duplicate = false
		row.Size = stat.Size()
		row.Unmanaged = !known
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		h := sha256.New()
		_, err = io.Copy(h, scanReader{ctx, f})
		f.Close()
		if err != nil {
			return err
		}
		hash := fmt.Sprintf("%x", h.Sum(nil))
		hashes[hash] = append(hashes[hash], len(rows))
		tags, err := ytdlp.Tags(ctx, p)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			row.Note = "Unreadable tags: " + err.Error()
		} else {
			if tags["title"] != "" {
				row.Title = tags["title"]
			}
			row.Creators = tags["artist"]
			row.Sources = tags["album"]
		}
		key := strings.ToLower(strings.Join(strings.Fields(row.Creators+" / "+row.Title), " "))
		titles[key] = append(titles[key], len(rows))
		rows = append(rows, row)
		return nil
	})
	for _, g := range titles {
		if len(g) > 1 {
			for _, i := range g {
				rows[i].Duplicate = true
				rows[i].Note += " · Same artist/title; compare editions"
			}
		}
	}
	for _, g := range hashes {
		if len(g) > 1 {
			for _, i := range g {
				rows[i].Duplicate = true
				rows[i].Note += " · Identical file bytes (SHA-256)"
			}
		}
	}
	sort.Slice(rows, func(i, j int) bool { return strings.ToLower(rows[i].Title) < strings.ToLower(rows[j].Title) })
	return rows, err
}

type scanReader struct {
	ctx context.Context
	r   io.Reader
}

func (r scanReader) Read(b []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(b)
}
