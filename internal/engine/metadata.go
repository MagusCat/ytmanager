package engine

import (
	"fmt"
	"regexp"
	"strings"
)

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
