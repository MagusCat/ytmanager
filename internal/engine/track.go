package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"ytmanager/internal/lyrics"
	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

// maxFails is how many syncs in a row a track may fail with an unrecognised, non-network error
// before it is treated as unavailable.
const maxFails = 3

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
		if err == nil || ctx.Err() != nil || ytdlp.Unavailable(err) {
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
	if moved { // re-downloaded by Reorganize into a new name
		removeOld(root, old)
	}

	switch {
	case saveErr != nil:
		emit(Event{Kind: EvFailed, ID: id, Title: title, Msg: "could not save data.music: " + saveErr.Error()})
	case status == manifest.StatusUnavailable:
		emit(Event{Kind: EvUnavailable, ID: id, Title: title, Msg: err.Error()})
	case err != nil:
		emit(Event{Kind: EvFailed, ID: id, Title: title, Msg: err.Error()})
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
	default:
		t.Status, t.Error = manifest.StatusFailed, err.Error()
		if !ytdlp.Transient(err) {
			if t.Fails++; t.Fails >= maxFails {
				t.Status = manifest.StatusUnavailable
			}
		}
	}
}

// download enriches the info-json (artist, album, track number, lyrics) and hands it to yt-dlp.
func (e *Engine) download(ctx context.Context, id, album string, n int, format, root string, progress func(float64)) (string, string, error) {
	info, err := ytdlp.Info(ctx, id, e.opt.Cookies)
	if err != nil {
		return "", "", err
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
		return "", "", err
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

// videoNoise matches the suffixes of uploads that are not YT Music tracks, e.g. "(Official Video)".
var videoNoise = regexp.MustCompile(`(?i)\s*[\(\[](official|lyric|audio|video|visualizer|hd|4k)[^\)\]]*[\)\]]`)

// tag normalises the metadata yt-dlp writes into the file and uses in the output path, so music
// apps group every track of an album together whatever source it came from: one album artist
// (the first artist, without YouTube's " - Topic" suffix) that also names the folder, the release
// year instead of the upload date, the saved album's name and track number, a clean title, and
// no genre guessed from video tags. opt.Artists decides where the other artists go. It returns
// the artist and title.
func tag(info map[string]any, album string, n int, opt Options) (artist, title string) {
	str := func(k string) string { s, _ := info[k].(string); return s }

	var all []string
	if as, ok := info["artists"].([]any); ok {
		for _, a := range as {
			if a, ok := a.(string); ok {
				all = append(all, a)
			}
		}
	}
	artist = strings.TrimSuffix(str("uploader"), " - Topic")
	if len(all) > 0 {
		artist = all[0]
	} else if a := str("artist"); a != "" {
		artist = a
	}
	if opt.Artists != "all" || str("artist") == "" {
		info["artist"] = artist
	}
	if str("album_artist") == "" {
		info["album_artist"] = artist
	}
	if y, ok := info["release_year"].(float64); ok {
		info["upload_date"] = fmt.Sprint(int(y))
	}
	if str("genre") == "" {
		info["genre"] = "" // stops yt-dlp falling back to the video's categories and tags
	}
	if album != "" {
		info["album"] = album
		info["track_number"] = n
	}
	info["ytm_prefix"] = ""
	switch tn := info["track_number"].(type) {
	case int:
		info["ytm_prefix"] = fmt.Sprintf("%02d - ", tn)
	case float64:
		info["ytm_prefix"] = fmt.Sprintf("%02d - ", int(tn))
	}
	if title = str("track"); title == "" {
		title = str("title")
		if !opt.RawTitles { // a plain video: "Artist - Song (Official Video)" becomes "Song"
			title = strings.TrimPrefix(videoNoise.ReplaceAllString(title, ""), artist+" - ")
		}
	}
	if opt.Artists == "" && len(all) > 1 && !strings.Contains(strings.ToLower(title), "feat") {
		title += " (feat. " + strings.Join(all[1:], ", ") + ")"
	}
	info["track"] = title
	return artist, title
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
