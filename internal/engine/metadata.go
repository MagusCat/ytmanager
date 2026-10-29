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

	all := creditedArtists(info)
	artist = "Unknown Artist"
	if len(all) > 0 {
		artist = all[0]
	}
	info["artist"] = artist
	if opt.Artists == "all" && len(all) > 0 {
		info["artist"] = strings.Join(all, ", ")
	}
	albumArtist := cleanArtist(str("album_artist"))
	// Keep explicit album credits, including compilations and guest tracks.
	// When absent, derive the folder artist from music credits, never the channel.
	if albumArtist == "" {
		albumArtist = artist
	}
	info["album_artist"] = albumArtist
	if y, ok := info["release_year"].(float64); ok {
		// Keep upload_date in yt-dlp's YYYYMMDD format. Override only the audio tag.
		info["meta_date"] = fmt.Sprint(int(y))
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

func cleanArtist(s string) string {
	return strings.Join(strings.Fields(strings.TrimSuffix(strings.TrimSpace(s), " - Topic")), " ")
}

// creditedArtists never splits names on commas or '&': those can be part of a
// band's name. Only a structured list identifies separate collaborators safely.
func creditedArtists(info map[string]any) []string {
	var out []string
	seen := map[string]bool{}
	add := func(name string) {
		name = cleanArtist(name)
		key := strings.ToLower(name)
		if name != "" && !seen[key] {
			out = append(out, name)
			seen[key] = true
		}
	}
	switch artists := info["artists"].(type) {
	case []any:
		for _, value := range artists {
			if name, ok := value.(string); ok {
				add(name)
			}
		}
	case []string:
		for _, name := range artists {
			add(name)
		}
	}
	if len(out) > 0 {
		return out
	}
	if artist, _ := info["artist"].(string); strings.TrimSpace(artist) != "" {
		add(artist)
		if len(out) > 0 {
			return out
		}
	}
	if albumArtist, _ := info["album_artist"].(string); strings.TrimSpace(albumArtist) != "" {
		add(albumArtist)
		if len(out) > 0 {
			return out
		}
	}
	// YouTube's auto-generated Topic channels are the only channel fallback.
	// Label, VEVO and fan-upload channel names are not artist credits.
	for _, key := range []string{"channel", "uploader"} {
		name, _ := info[key].(string)
		if strings.HasSuffix(strings.TrimSpace(name), " - Topic") {
			add(name)
			if len(out) > 0 {
				return out
			}
		}
	}
	return out
}
