package ytdlp

import (
	"errors"
	"testing"
)

func TestKeyring(t *testing.T) {
	cases := []struct{ browser, goos, desktop, want string }{
		{"brave", "linux", "Wayfire:wlroots", "brave+gnomekeyring"},
		{"brave", "linux", "KDE", "brave"},
		{"firefox", "linux", "Wayfire", "firefox"},
		{"chrome+kwallet6", "linux", "Wayfire", "chrome+kwallet6"},
		{"brave", "windows", "", "brave"},
		{"chrome", "darwin", "", "chrome"},
	}
	for _, c := range cases {
		if got := Keyring(c.browser, c.goos, c.desktop); got != c.want {
			t.Errorf("Keyring(%q, %q, %q) = %q, want %q", c.browser, c.goos, c.desktop, got, c.want)
		}
	}
}

func TestUnavailable(t *testing.T) {
	yes := []string{"[youtube] aaaaaaaaaaa: This video is unavailable", "Video unavailable. This video has been removed by the uploader", "Private video. Sign in if you've been granted access", "The uploader has not made this video available in your country. This video is not available"}
	no := []string{"Requested format is not available", "Unable to download webpage: timed out"}
	for _, s := range yes {
		if !Unavailable(errors.New(s)) {
			t.Errorf("should be unavailable: %q", s)
		}
	}
	for _, s := range no {
		if Unavailable(errors.New(s)) {
			t.Errorf("should not be unavailable: %q", s)
		}
	}
}

func TestMatch(t *testing.T) {
	album := []Entry{
		{Title: "Aquí Estoy Yo (feat. Andrea Echeverri)", Duration: 200},
		{Title: "El Tiempo Pasa", Duration: 180},
		{Title: "Se Nos Perdió", Duration: 210},
	}
	cases := []struct {
		title string
		dur   float64
		want  int
	}{
		{"El Tiempo Pasa (Official Video)", 0, 1},
		{"Aqui Estoy Yo ft. Andrea Echeverri [Remastered]", 0, 0}, // accents and featuring ignored
		{"Se Nos Perdio - Live", 211, 2},                          // duration breaks the tie
		{"Robot", 0, -1},
	}
	for _, c := range cases {
		if got := Match(c.title, c.dur, album); got != c.want {
			t.Errorf("Match(%q) = %d, want %d", c.title, got, c.want)
		}
	}
}

func TestTransient(t *testing.T) {
	if !Transient(errors.New("Unable to download webpage: The read operation timed out")) {
		t.Error("timeout should be transient")
	}
	if Transient(errors.New("Requested format is not available")) {
		t.Error("format error is not transient")
	}
}

func TestKind(t *testing.T) {
	cases := map[string]string{
		"https://music.youtube.com/playlist?list=OLAK5uy_x": "album",
		"https://music.youtube.com/playlist?list=RDTMAKx":   "mix",
		"https://music.youtube.com/playlist?list=RDCLAKx":   "playlist",
		"https://music.youtube.com/playlist?list=LM":        "playlist",
		"https://www.youtube.com/watch?v=x":                 "track",
	}
	for url, want := range cases {
		if got := Kind(url); got != want {
			t.Errorf("Kind(%q) = %q, want %q", url, got, want)
		}
	}
}
