package engine

import (
	"context"
	"errors"
	"testing"

	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

func TestSourceReadLifecycle(t *testing.T) {
	for _, mode := range []string{"success", "failure", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			e := queueEngine(t, 1)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			e.listSource = func(context.Context, string, string) (*ytdlp.Listing, error) {
				if mode == "cancel" {
					cancel()
					return nil, context.Canceled
				}
				if mode == "failure" {
					return nil, errors.New("lookup failed")
				}
				return listing("album", "song"), nil
			}
			var lifecycle []Event
			e.readSource(ctx, manifest.SourceRequest{URL: "link"}, func(ev Event) {
				if ev.Kind == EvReading || ev.Kind == EvReadDone {
					lifecycle = append(lifecycle, ev)
				}
			})
			if len(lifecycle) != 2 || lifecycle[0].Kind != EvReading || lifecycle[1].Kind != EvReadDone || lifecycle[0].ID != "link" || lifecycle[1].ID != "link" {
				t.Fatalf("unbalanced reading events: %+v", lifecycle)
			}
		})
	}
}
