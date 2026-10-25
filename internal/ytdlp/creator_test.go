package ytdlp

import "testing"

// Flat album entries carry artists even when the playlist itself has no artist.
// A regular playlist must keep its curator rather than borrowing a song's artist.
func TestListingCreator(t *testing.T) {
	cases := []struct {
		name, url, data, want string
	}{
		{"album entries", "https://music.youtube.com/playlist?list=OLAK5uy_test",
			`{"_type":"playlist","id":"album","title":"Album - Greatest Hits","uploader":"YouTube Music","entries":[
				{"id":"a","artists":["Artist A"]},{"id":"b","artists":["Artist A"]},
				{"id":"c","artist":"Artist B"},{"id":"p","title":"[Private video]","channel":"Unknown"}]}`,
			"Artist A, Artist B"},
		{"album metadata", "https://music.youtube.com/playlist?list=OLAK5uy_test",
			`{"_type":"playlist","album_artist":"Album Artist","entries":[{"id":"a","artist":"Guest Artist"}]}`,
			"Album Artist"},
		{"album channel fallback", "https://music.youtube.com/playlist?list=OLAK5uy_test",
			`{"_type":"playlist","channel":"Record Label","entries":[{"id":"a"}]}`,
			"Record Label"},
		{"playlist owner", "https://www.youtube.com/playlist?list=PLtest",
			`{"_type":"playlist","uploader":"Curator","artist":"Song Artist","entries":[{"id":"a","artist":"Song Artist"}]}`,
			"Curator"},
		{"mix owner", "https://music.youtube.com/playlist?list=RDtest",
			`{"_type":"playlist","channel":"Mix Channel","entries":[{"id":"a","artist":"Song Artist"}]}`,
			"Mix Channel"},
		{"song artists", "https://www.youtube.com/watch?v=a",
			`{"id":"a","artists":[" Artist A ","Artist B","artist a",""],"channel":"Video Channel"}`,
			"Artist A, Artist B"},
		{"song channel", "https://www.youtube.com/watch?v=a",
			`{"id":"a","channel":"Video Channel","uploader":"Uploader"}`, "Video Channel"},
		{"song uploader", "https://www.youtube.com/watch?v=a",
			`{"id":"a","uploader":"Uploader"}`, "Uploader"},
		{"unknown playlist", "https://www.youtube.com/playlist?list=PLtest",
			`{"_type":"playlist","entries":[{"id":"a","artist":"Song Artist"}]}`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			l, err := parseListing([]byte(c.data), c.url)
			if err != nil {
				t.Fatal(err)
			}
			if l.Creator != c.want {
				t.Fatalf("creator = %q, want %q", l.Creator, c.want)
			}
		})
	}
}
