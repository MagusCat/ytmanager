package lyrics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type lrc struct {
	Plain  string `json:"plainLyrics"`
	Synced string `json:"syncedLyrics"`
}

func (l lrc) result() (Result, bool) {
	if l.Synced != "" {
		return Result{l.Synced, true}, true
	}
	return Result{Text: l.Plain}, l.Plain != ""
}

func lrclib(ctx context.Context, artist, title, album string, duration float64) (Result, bool) {
	q := url.Values{"artist_name": {artist}, "track_name": {title}}
	if album != "" {
		q.Set("album_name", album)
	}
	if duration > 0 {
		q.Set("duration", fmt.Sprint(int(duration)))
	}
	var one lrc
	if getJSON(ctx, "https://lrclib.net/api/get?"+q.Encode(), &one) == nil {
		if r, ok := one.result(); ok {
			return r, true
		}
	}
	var many []lrc
	if getJSON(ctx, "https://lrclib.net/api/search?"+url.Values{"track_name": {title}, "artist_name": {artist}}.Encode(), &many) == nil {
		for _, l := range many {
			if r, ok := l.result(); ok {
				return r, true
			}
		}
	}
	return Result{}, false
}

func getJSON(ctx context.Context, u string, v any) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	req.Header.Set("User-Agent", "ytmanager/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
