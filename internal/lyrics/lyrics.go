// Package lyrics finds lyrics: LRCLIB first (synced), then YT Music (plain text).
package lyrics

import (
	"context"
	"net/http"
	"time"
)

type Result struct {
	Text   string
	Synced bool
}

var client = &http.Client{Timeout: 15 * time.Second}

func Find(ctx context.Context, artist, title, album string, duration float64, videoID string) (Result, bool) {
	if r, ok := lrclib(ctx, artist, title, album, duration); ok {
		return r, true
	}
	if txt, err := ytmusic(ctx, videoID); err == nil && txt != "" {
		return Result{Text: txt}, true
	}
	return Result{}, false
}
