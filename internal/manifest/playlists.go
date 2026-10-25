package manifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// WritePlaylists writes one .m3u8 per playlist, album or mix with relative paths, for car and phone players.
func (m *Manifest) WritePlaylists() error {
	for _, s := range m.Sources {
		if s.Type == "track" {
			continue
		}
		var b strings.Builder
		b.WriteString("#EXTM3U\n")
		for _, id := range s.Tracks {
			if t := m.Tracks[id]; t.Status == StatusDone {
				fmt.Fprintf(&b, "#EXTINF:%d,%s\n%s\n", int(t.Duration), t.Title, filepath.ToSlash(t.Path))
			}
		}
		if err := os.WriteFile(filepath.Join(m.Root, SafeName(s.Title)+".m3u8"), []byte(b.String()), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// SafeName returns a file name that is valid on FAT32/exFAT.
func SafeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.Trim(s, " .")
	if r := []rune(s); len(r) > 120 {
		s = string(r[:120])
	}
	if s == "" {
		s = "untitled"
	}
	return s
}
