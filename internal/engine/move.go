package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"ytmanager/internal/device"
	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

// Rearrange moves every downloaded file to where the current Folders setting puts it, named from
// its own tags, so changing the layout needs no new download. Each file is reported as a track:
// EvDone once it is in place, EvFailed if it could not be moved.
func (e *Engine) Rearrange(ctx context.Context, emit func(Event)) error {
	e.mu.Lock()
	var ids []string
	for id, t := range e.m.Tracks {
		if t.Status == manifest.StatusDone {
			ids = append(ids, id)
		}
	}
	e.mu.Unlock()
	if len(ids) > 0 {
		for _, tool := range []string{"ffprobe", "yt-dlp"} {
			if _, err := exec.LookPath(tool); err != nil {
				install := "install yt-dlp"
				if tool == "ffprobe" {
					install = "install ffmpeg with ffprobe"
				}
				return fmt.Errorf("cannot move files: %s is missing; %s and try again: %w", tool, install, err)
			}
		}
	}
	emit(Event{Kind: EvPlan, Total: len(ids)})

	work := make(chan string)
	var wg sync.WaitGroup
	jobs := e.opt.Jobs
	if device.IsMTP(e.m.Root) {
		jobs = 1
	}
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				e.worker(ctx, id, emit, e.move)
			}
		}()
	}
feed:
	for _, id := range ids {
		select {
		case work <- id:
		case <-ctx.Done():
			break feed
		}
	}
	close(work)
	wg.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()
	if err := e.m.WritePlaylists(); err != nil {
		return err
	}
	if err := e.m.Save(); err != nil {
		return err
	}
	return ctx.Err()
}

// move puts one downloaded track where the layout says; it returns false if ctx stopped it.
func (e *Engine) move(ctx context.Context, id string, emit func(Event)) bool {
	e.mu.Lock()
	t := e.m.Tracks[id]
	title, old, root := t.Title, t.Path, e.m.Root
	e.mu.Unlock()

	emit(Event{Kind: EvStart, ID: id, Title: title})
	src := filepath.Join(root, old)
	tags, err := ytdlp.Tags(ctx, src)
	if ctx.Err() == nil && (errors.Is(err, ytdlp.ErrMissingAudio) || errors.Is(err, ytdlp.ErrInvalidAudio)) {
		// Sync restores only these tracks when the user requests it; moving never
		// silently downloads replacements or removes the original file.
		if _, rootErr := os.Stat(root); rootErr != nil {
			err = fmt.Errorf("destination is unavailable; reconnect it and retry: %w", rootErr)
		} else {
			e.mu.Lock()
			t.Status, t.Error = manifest.StatusPending, err.Error()
			saveErr := e.m.Save()
			e.mu.Unlock()
			if saveErr != nil {
				err = fmt.Errorf("%w; could not save repair state: %v", err, saveErr)
			}
		}
	}
	var rel string
	if err == nil {
		if tags["title"] == "" {
			tags["title"] = title
		}
		rel, err = ytdlp.Name(ctx, tags, strings.TrimPrefix(filepath.Ext(old), "."), e.opt.Folders)
	}
	if ctx.Err() != nil {
		emit(Event{Kind: EvStopped, ID: id, Title: title})
		return false
	}
	if err == nil && rel != old {
		if err = moveFile(root, old, rel); err == nil {
			e.mu.Lock()
			t.Path = rel
			err = e.m.Save()
			e.mu.Unlock()
		}
	}
	if err != nil {
		emit(Event{Kind: EvFailed, ID: id, Title: title, Msg: err.Error()})
	} else {
		emit(Event{Kind: EvDone, ID: id, Title: title})
	}
	return true
}

// moveFile moves root/old to root/rel with its .lrc and removes the folders left empty. It never
// overwrites: another file at rel is an error.
func moveFile(root, old, rel string) error {
	src, dst := filepath.Join(root, old), filepath.Join(root, rel)
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("%s already exists", rel)
	}
	if err := relocate(src, dst); err != nil {
		return err
	}
	lrc := func(p string) string { return strings.TrimSuffix(p, filepath.Ext(p)) + ".lrc" }
	if _, err := os.Stat(lrc(src)); err == nil {
		_ = relocate(lrc(src), lrc(dst)) // lyrics are still embedded in the audio
	}
	removeOld(root, old)
	return nil
}

// relocate renames src to dst. MTP (gvfs) loses files on rename, so there it copies and deletes.
func relocate(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if !device.IsMTP(src) && os.Rename(src, dst) == nil {
		return nil
	}
	if err := ytdlp.CopyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

// removeOld deletes a track's previous file, its .lrc and the folders it leaves empty.
func removeOld(root, rel string) {
	p := filepath.Join(root, rel)
	os.Remove(p)
	os.Remove(strings.TrimSuffix(p, filepath.Ext(p)) + ".lrc")
	for d := filepath.Dir(p); len(d) > len(root); d = filepath.Dir(d) {
		if os.Remove(d) != nil {
			break // not empty
		}
	}
}
