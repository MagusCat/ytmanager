package ytdlp

import "strings"

type creatorMetadata struct {
	AlbumArtist string   `json:"album_artist"`
	Artists     []string `json:"artists"`
	Artist      string   `json:"artist"`
	Channel     string   `json:"channel"`
	Uploader    string   `json:"uploader"`
}

func (m creatorMetadata) artist() string {
	if s := strings.TrimSpace(m.AlbumArtist); s != "" {
		return s
	}
	if s := uniqueNames(m.Artists); s != "" {
		return s
	}
	return strings.TrimSpace(m.Artist)
}

func (m creatorMetadata) creator() string {
	if s := m.artist(); s != "" {
		return s
	}
	return m.owner()
}

func (m creatorMetadata) owner() string {
	if s := strings.TrimSpace(m.Channel); s != "" {
		return s
	}
	return strings.TrimSpace(m.Uploader)
}

func listingCreator(kind string, meta creatorMetadata, entries []Entry) string {
	if kind != "album" {
		// A playlist's owner is distinct from the artists whose songs it contains.
		if s := meta.owner(); s != "" {
			return s
		}
		return meta.artist()
	}
	if s := meta.artist(); s != "" {
		return s
	}
	var artists []string
	for _, e := range entries {
		if !e.Unavailable {
			artists = append(artists, e.creatorMetadata.creator())
		}
	}
	if s := uniqueNames(artists); s != "" {
		return s
	}
	return meta.owner()
}

func uniqueNames(names []string) string {
	seen := map[string]bool{}
	var out []string
	for _, name := range names {
		name = strings.TrimSpace(name)
		if key := strings.ToLower(name); name != "" && !seen[key] {
			seen[key] = true
			out = append(out, name)
		}
	}
	return strings.Join(out, ", ")
}
