package engine

import "testing"

func TestArtistCreditsIgnoreUploadChannel(t *testing.T) {
	for _, channel := range []string{"Label Records", "Fan Uploads", "SingerVEVO", "Singer - Topic"} {
		info := map[string]any{
			"title": "Song", "artists": []any{" Singer ", "", "singer", "Guest"},
			"artist": channel, "uploader": channel,
		}
		artist, title := tag(info, "Album", 1, Options{})
		if artist != "Singer" || info["artist"] != "Singer" || info["album_artist"] != "Singer" || title != "Song (feat. Guest)" {
			t.Fatalf("channel %q changed credits: %+v", channel, info)
		}
	}
}

func TestArtistMetadataFallbacks(t *testing.T) {
	for _, tc := range []struct {
		name                string
		info                map[string]any
		artist, albumArtist string
	}{
		{"ordinary uploader is not artist", map[string]any{"uploader": "Record Label"}, "Unknown Artist", "Unknown Artist"},
		{"VEVO name is not guessed", map[string]any{"uploader": "SingerVEVO"}, "Unknown Artist", "Unknown Artist"},
		{"topic fallback", map[string]any{"uploader": " Singer - Topic "}, "Singer", "Singer"},
		{"empty list uses credit", map[string]any{"artists": []any{"", "  ", nil}, "artist": " Singer "}, "Singer", "Singer"},
		{"typed list", map[string]any{"artists": []string{" Singer ", "singer"}}, "Singer", "Singer"},
		{"album credit", map[string]any{"album_artist": "Singer", "uploader": "Label"}, "Singer", "Singer"},
		{"compilation", map[string]any{"artist": "Singer", "album_artist": "Various Artists"}, "Singer", "Various Artists"},
		{"band name", map[string]any{"artist": "Earth, Wind & Fire"}, "Earth, Wind & Fire", "Earth, Wind & Fire"},
		{"album artist owns channel", map[string]any{"artist": "Guest", "album_artist": "Singer", "uploader": "Singer"}, "Guest", "Singer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.info["title"] = "Song"
			artist, _ := tag(tc.info, "", 0, Options{})
			if artist != tc.artist || tc.info["album_artist"] != tc.albumArtist {
				t.Fatalf("got %q / %v; want %q / %q", artist, tc.info["album_artist"], tc.artist, tc.albumArtist)
			}
		})
	}
}

func TestAllArtistsUsesCleanCredits(t *testing.T) {
	info := map[string]any{"title": "Song", "artist": "Label", "artists": []any{"Singer", "Guest", " guest "}}
	tag(info, "", 0, Options{Artists: "all"})
	if info["artist"] != "Singer, Guest" || info["album_artist"] != "Singer" || info["track"] != "Song" {
		t.Fatalf("all artists: %+v", info)
	}
}
