package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ytmanager/internal/device"
	"ytmanager/internal/lyrics"
	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

// track downloads one track and records the result; it returns false if ctx stopped it midway.
func (e *Engine) track(ctx context.Context, id string, emit func(Event)) bool {
	e.mu.Lock()
	t := e.m.Tracks[id]
	title, root, format := t.Title, e.m.Root, e.m.Format
	dl := id // an alternative is a different video, but it keeps id's slot (album, number, playlist)
	if t.Alt != "" {
		dl = t.Alt
	}
	album, n := e.m.AlbumOf(id)
	e.mu.Unlock()

	emit(Event{Kind: EvStart, ID: id, Title: title})
	var path, lyr string
	var err error
	for attempt := 0; attempt < 2; attempt++ {
		path, lyr, err = e.download(ctx, dl, album, n, format, root, func(p float64) {
			emit(Event{Kind: EvProgress, ID: id, Title: title, Pct: p})
		})
		if err == nil || ctx.Err() != nil || ytdlp.Unavailable(err) || device.IsSpaceError(err) {
			break
		}
	}
	if ctx.Err() != nil {
		emit(Event{Kind: EvStopped, ID: id, Title: title})
		return false
	}

	e.mu.Lock()
	old := t.Path
	record(t, root, path, lyr, err)
	status, moved := t.Status, old != "" && old != t.Path && t.Status == manifest.StatusDone
	saveErr := e.m.Save()
	e.mu.Unlock()
	if moved && saveErr == nil { // keep the old copy if the new path was not saved
		removeOld(root, old)
	}

	switch {
	case saveErr != nil:
		emit(Event{Kind: EvFailed, ID: id, Title: title, Msg: "could not save data.music: " + saveErr.Error(), Err: saveErr})
	case status == manifest.StatusUnavailable:
		emit(Event{Kind: EvUnavailable, ID: id, Title: title, Msg: err.Error()})
	case err != nil:
		emit(Event{Kind: EvFailed, ID: id, Title: title, Msg: err.Error(), Err: err})
	default:
		emit(Event{Kind: EvDone, ID: id, Title: title, Msg: lyr})
	}
	return true
}

func record(t *manifest.Track, root, path, lyr string, err error) {
	switch {
	case err == nil:
		rel, _ := filepath.Rel(root, path)
		t.Status, t.Path, t.Lyrics, t.Error, t.Fails = manifest.StatusDone, rel, lyr, "", 0
	case ytdlp.Unavailable(err):
		t.Status, t.Error = manifest.StatusUnavailable, err.Error()
	case device.IsSpaceError(err):
		t.Status, t.Error = manifest.StatusPending, "Not enough free space; free space and sync to resume."
	default:
		t.Status, t.Error = manifest.StatusFailed, err.Error()
		if !ytdlp.Transient(err) {
			t.Fails++ // an unknown failure does not prove that a video is unavailable
		}
	}
}

// download enriches the info-json (artist, album, track number, lyrics) and hands it to yt-dlp.
func (e *Engine) download(ctx context.Context, id, album string, n int, format, root string, progress func(float64)) (string, string, error) {
	info, err := ytdlp.Info(ctx, id, e.opt.Cookies)
	if err != nil {
		return "", "", fmt.Errorf("reading song information: %w", err)
	}
	artist, title := tag(info, album, n, e.opt)
	dur, _ := info["duration"].(float64)
	album, _ = info["album"].(string)

	lr, hasLyr := lyrics.Find(ctx, artist, title, album, dur, id)
	if hasLyr {
		info["meta_lyrics"] = lr.Text
	}

	tmp, err := os.CreateTemp("", "ytm-*.info.json")
	if err != nil {
		return "", "", err
	}
	defer os.Remove(tmp.Name())
	err = json.NewEncoder(tmp).Encode(info)
	tmp.Close()
	if err != nil {
		return "", "", err
	}

	path, err := ytdlp.Download(ctx, id, tmp.Name(), root, format, e.opt.Folders, e.opt.Cookies, progress)
	if err != nil {
		return "", "", fmt.Errorf("downloading or processing audio: %w", err)
	}
	kind := ""
	if hasLyr {
		kind = "plain"
		if lr.Synced {
			kind = "synced"
			// .lrc next to the audio: most players read it for synced lyrics
			_ = os.WriteFile(strings.TrimSuffix(path, filepath.Ext(path))+".lrc", []byte(lr.Text), 0o644)
		}
	}
	return path, kind, nil
}
