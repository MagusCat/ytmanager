package engine

import (
	"context"
	"testing"
	"time"

	"ytmanager/internal/ytdlp"
)

func TestParallelAlbumsShareWorkersAndReportSources(t *testing.T) {
	e := queueEngine(t, 2)
	opt := e.Options()
	opt.AlbumJobs = 2
	e.SetOptions(opt)
	started := make(chan Event, 8)
	release := make(chan struct{})
	read := make(chan string, 4)
	e.listSource = func(ctx context.Context, url, cookies string) (*ytdlp.Listing, error) {
		read <- url
		return listing(url, url+"-song"), nil
	}
	e.trackWork = func(ctx context.Context, id string, emit func(Event)) bool {
		emit(Event{Kind: EvStart, ID: id, Title: id})
		select {
		case <-release:
		case <-ctx.Done():
			return false
		}
		return finishTrack(t, e, ctx, id, emit)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- e.Download(ctx, []string{"first", "second", "third"}, func(ev Event) {
			if ev.Kind == EvStart {
				started <- ev
			}
		})
	}()
	for i := 0; i < 2; i++ {
		select {
		case ev := <-started:
			if ev.SourceID == "" || ev.SourceTitle != ev.SourceID || ev.ID != ev.SourceID+"-song" {
				t.Errorf("missing source context: %+v", ev)
			}
		case <-ctx.Done():
			t.Fatal("two albums did not download concurrently")
		}
	}
	// Both album slots and both workers are held; a third source must wait.
	<-read
	<-read
	select {
	case url := <-read:
		t.Errorf("read third album before a slot freed: %s", url)
	default:
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if url := <-read; url != "third" {
		t.Fatalf("next album = %s", url)
	}
}
