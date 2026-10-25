package lyrics

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
)

// fallbackVersion is used when the current client version cannot be read from YT Music's page.
const fallbackVersion = "1.20261020.10.00"

var (
	versionOnce sync.Once
	version     string
	versionRe   = regexp.MustCompile(`"INNERTUBE_CLIENT_VERSION":"([^"]+)"`)
)

// clientVersion reads the innertube client version YT Music currently serves, once per run,
// because old versions eventually get rejected.
func clientVersion(ctx context.Context) string {
	versionOnce.Do(func() {
		version = fallbackVersion
		req, _ := http.NewRequestWithContext(ctx, "GET", "https://music.youtube.com/", nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130 Safari/537.36")
		req.Header.Set("Cookie", "SOCS=CAI") // skips the EU consent page
		resp, err := client.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		page, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		if m := versionRe.FindSubmatch(page); m != nil {
			version = string(m[1])
		}
	})
	return version
}

// ytmusic uses YT Music's internal (innertube) API: next -> lyrics tab -> browse.
func ytmusic(ctx context.Context, videoID string) (string, error) {
	yctx := map[string]any{"client": map[string]any{"clientName": "WEB_REMIX", "clientVersion": clientVersion(ctx), "hl": "en"}}
	next, err := innertube(ctx, "next", map[string]any{"context": yctx, "videoId": videoID})
	if err != nil {
		return "", err
	}
	var browseID string
	walk(next, func(k string, v any) bool {
		if s, ok := v.(string); ok && k == "browseId" && strings.HasPrefix(s, "MPLY") {
			browseID = s
			return true
		}
		return false
	})
	if browseID == "" {
		return "", fmt.Errorf("no lyrics on YT Music")
	}
	page, err := innertube(ctx, "browse", map[string]any{"context": yctx, "browseId": browseID})
	if err != nil {
		return "", err
	}
	var text string
	walk(page, func(k string, v any) bool {
		if k != "musicDescriptionShelfRenderer" {
			return false
		}
		walk(v, func(k string, v any) bool {
			if k != "description" {
				return false
			}
			walk(v, func(k string, v any) bool {
				if s, ok := v.(string); ok && k == "text" {
					text += s
				}
				return false
			})
			return true
		})
		return true
	})
	return strings.TrimSpace(text), nil
}

func innertube(ctx context.Context, endpoint string, body any) (any, error) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://music.youtube.com/youtubei/v1/"+endpoint+"?prettyPrint=false", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://music.youtube.com")
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("youtubei %s: http %d", endpoint, resp.StatusCode)
	}
	var v any
	return v, json.NewDecoder(resp.Body).Decode(&v)
}

// walk visits the JSON depth-first; fn returns true to stop descending that branch.
func walk(v any, fn func(k string, v any) bool) bool {
	switch t := v.(type) {
	case map[string]any:
		for k, c := range t {
			if fn(k, c) {
				return true
			}
			if walk(c, fn) {
				return true
			}
		}
	case []any:
		for _, c := range t {
			if walk(c, fn) {
				return true
			}
		}
	}
	return false
}
