package ytdlp

import (
	"math"
	"regexp"
	"strings"
	"unicode"
)

// Keyring adds +gnomekeyring to Chromium browsers on Linux outside KDE: yt-dlp picks the keyring
// from XDG_CURRENT_DESKTOP and cannot decrypt cookies on desktops it does not know (Wayfire,
// Hyprland…). macOS and Windows have a single system keychain that yt-dlp finds on its own.
func Keyring(browser, goos, desktop string) string {
	chromium := map[string]bool{"chrome": true, "chromium": true, "brave": true, "edge": true, "vivaldi": true, "opera": true}
	if goos != "linux" || strings.Contains(browser, "+") || !chromium[browser] || strings.Contains(strings.ToUpper(desktop), "KDE") {
		return browser
	}
	return browser + "+gnomekeyring"
}

func contains(err error, patterns []string) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// Unavailable reports whether a yt-dlp error is known to be permanent (video removed, private,
// blocked). Unknown permanent errors are caught by the engine after repeated failures.
func Unavailable(err error) bool {
	return contains(err, []string{
		"video unavailable", "video is unavailable", "this video is not available",
		"not available in your country", "private video", "has been removed",
		"has been terminated", "copyright", "no longer available",
	})
}

// Transient reports network and server errors, which never count toward marking a track unavailable.
func Transient(err error) bool {
	return contains(err, []string{
		"timed out", "timeout", "temporary failure", "connection", "network is unreachable",
		"name resolution", "getaddrinfo", "http error 5", "http error 429", "ssl", "eof occurred",
	})
}

var (
	bracketed = regexp.MustCompile(`\([^)]*\)|\[[^\]]*\]`)
	featuring = regexp.MustCompile(`(?i)\b(feat|ft)\..*`)
	accents   = strings.NewReplacer("á", "a", "à", "a", "ä", "a", "â", "a", "ã", "a", "é", "e", "è", "e", "ë", "e", "ê", "e",
		"í", "i", "ì", "i", "ï", "i", "î", "i", "ó", "o", "ò", "o", "ö", "o", "ô", "o", "õ", "o",
		"ú", "u", "ù", "u", "ü", "u", "û", "u", "ñ", "n", "ç", "c")
)

// words normalises a title for comparison: lowercase, no accents, and without the parts that
// differ between uploads of the same song ("(Official Video)", "[Remastered]", "feat. X").
func words(title string) map[string]bool {
	s := accents.Replace(strings.ToLower(title))
	s = featuring.ReplaceAllString(bracketed.ReplaceAllString(s, " "), " ")
	w := map[string]bool{}
	for _, f := range strings.FieldsFunc(s, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		w[f] = true
	}
	if len(w) == 0 { // the title was all brackets: compare the raw words instead
		for _, f := range strings.Fields(strings.ToLower(title)) {
			w[f] = true
		}
	}
	return w
}

// Match picks the album/playlist entry most similar to a track with this title and duration
// (0 = unknown). It returns -1 when no entry is a convincing match.
func Match(title string, duration float64, entries []Entry) int {
	want := words(title)
	best, bestScore := -1, 0.0
	for i, e := range entries {
		got := words(e.Title)
		common := 0
		for w := range want {
			if got[w] {
				common++
			}
		}
		score := float64(common) / float64(max(len(want), len(got), 1))
		if duration > 0 && e.Duration > 0 && math.Abs(duration-e.Duration) <= 5 {
			score += 0.25 // same length is strong evidence of the same recording
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	if bestScore < 0.5 {
		return -1
	}
	return best
}
