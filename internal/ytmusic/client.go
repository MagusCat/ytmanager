// Package ytmusic resolves the audio recording linked by YouTube Music.
package ytmusic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

var client = &http.Client{Timeout: 15 * time.Second}

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
	return v, json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&v)
}
