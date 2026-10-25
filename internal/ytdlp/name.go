package ytdlp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// naming makes file names valid on phones and Windows; Name must use the same flags.
var naming = []string{"--windows-filenames", "--trim-filenames", "180"}

// layouts are the output templates for each Options.Folders value.
// The artist and flat layouts omit the album; identical artist/title pairs share a path.
var layouts = map[string]string{
	"":       "%(album_artist|Unknown)s/%(album|Singles)s/%(ytm_prefix|)s%(track,title)s.%(ext)s",
	"artist": "%(album_artist|Unknown)s/%(track,title)s.%(ext)s",
	"flat":   "%(album_artist|Unknown)s - %(track,title)s.%(ext)s",
}

// Tags reads the metadata of an audio file with ffprobe; keys are lower case.
func Tags(ctx context.Context, path string) (map[string]string, error) {
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format_tags:stream_tags", "-of", "json", path).Output()
	if err != nil {
		return nil, fmt.Errorf("could not read the tags of %s: %w", filepath.Base(path), err)
	}
	var r struct {
		Format  struct{ Tags map[string]string }   `json:"format"`
		Streams []struct{ Tags map[string]string } `json:"streams"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, err
	}
	tags := map[string]string{}
	for _, s := range append(r.Streams, r.Format) { // opus keeps them in the stream, m4a and mp3 in the format
		for k, v := range s.Tags {
			tags[strings.ToLower(k)] = v
		}
	}
	return tags, nil
}

// Name returns the path, relative to the destination, that Download gives a file with these tags
// in the folders layout. yt-dlp builds it offline, so it matches new downloads exactly.
func Name(ctx context.Context, tags map[string]string, ext, folders string) (string, error) {
	if tags["title"] == "" {
		return "", errors.New("the file has no title tag")
	}
	info := map[string]any{"id": "x", "title": tags["title"], "track": tags["title"], "ext": ext, "ytm_prefix": "",
		"url": "http://localhost/x", "webpage_url": "http://localhost/x", "extractor": "generic", "extractor_key": "Generic"}
	for _, k := range []string{"album_artist", "album"} {
		if tags[k] != "" {
			info[k] = tags[k]
		}
	}
	if n, err := strconv.Atoi(strings.Split(tags["track"], "/")[0]); err == nil { // "2" or "2/12"
		info["ytm_prefix"] = fmt.Sprintf("%02d - ", n)
	}
	tmp, err := os.CreateTemp("", "ytm-*.info.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	err = json.NewEncoder(tmp).Encode(info)
	tmp.Close()
	if err != nil {
		return "", err
	}
	args := append([]string{"--no-warnings", "--load-info-json", tmp.Name(), "--skip-download", "-o", layouts[folders], "--print", "filename"}, naming...)
	out, err := output(ctx, args...)
	if err != nil {
		return "", err
	}
	return filepath.Clean(filepath.FromSlash(strings.TrimSpace(string(out)))), nil
}
