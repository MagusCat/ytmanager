package engine

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"ytmanager/internal/manifest"
	"ytmanager/internal/ytdlp"
)

func queueEngine(t *testing.T, jobs int) *Engine {
	t.Helper()
	e, err := Open(t.TempDir(), Options{Jobs: jobs})
	if err != nil {
		t.Fatal(err)
	}
	e.spaceCheck = func(string, uint64) error { return nil }
	e.trackWork = func(ctx context.Context, id string, emit func(Event)) bool {
		return finishTrack(t, e, ctx, id, emit)
	}
	return e
}

func finishTrack(t *testing.T, e *Engine, ctx context.Context, id string, emit func(Event)) bool {
	t.Helper()
	emit(Event{Kind: EvStart, ID: id, Title: id})
	if ctx.Err() != nil {
		return false
	}
	if err := os.WriteFile(filepath.Join(e.Root(), id+".opus"), []byte("audio"), 0o644); err != nil {
		t.Error(err)
		return true
	}
	e.mu.Lock()
	e.m.Tracks[id].Path, e.m.Tracks[id].Status = id+".opus", manifest.StatusDone
	e.mu.Unlock()
	emit(Event{Kind: EvDone, ID: id, Title: id})
	return true
}

func listing(id string, tracks ...string) *ytdlp.Listing {
	l := &ytdlp.Listing{ID: id, Type: "album", Title: id}
	for _, track := range tracks {
		l.Entries = append(l.Entries, ytdlp.Entry{ID: track, Title: track})
	}
	return l
}

func TestDownloadsEachAlbumBeforeReadingTheNext(t *testing.T) {
	e := queueEngine(t, 2)
	started, release := make(chan string, 3), make(chan struct{})
	e.trackWork = func(ctx context.Context, id string, emit func(Event)) bool {
		started <- id
		select {
		case <-release:
		case <-ctx.Done():
			return false
		}
		return finishTrack(t, e, ctx, id, emit)
	}
	e.listSource = func(ctx context.Context, raw, cookies string) (*ytdlp.Listing, error) {
		if raw == "first" {
			return listing("first", "a1", "a2"), nil
		}
		e.mu.Lock()
		bothDone := e.m.Tracks["a1"].Status == manifest.StatusDone && e.m.Tracks["a2"].Status == manifest.StatusDone
		e.mu.Unlock()
		if !bothDone {
			t.Error("read the second album before finishing the first")
		}
		return listing("second", "b1"), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- e.Download(ctx, []string{"first", "second"}, func(Event) {}) }()
	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			t.Fatal("track workers did not run in parallel within the album")
		}
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if e.QueuedSources() != 0 {
		t.Fatal("completed requests left in queue")
	}
}

func TestResumeSavedTracksBeforeQueuedAndNewLinks(t *testing.T) {
	e := queueEngine(t, 2)
	e.m.Merge("album", "saved", "Saved", "saved-url", []manifest.Entry{{ID: "old"}})
	e.m.Queue = []manifest.SourceRequest{{URL: "queued"}}
	var mu sync.Mutex
	var trace []string
	e.trackWork = func(ctx context.Context, id string, emit func(Event)) bool {
		ok := finishTrack(t, e, ctx, id, emit)
		mu.Lock()
		trace = append(trace, "done:"+id)
		mu.Unlock()
		return ok
	}
	e.listSource = func(ctx context.Context, raw, cookies string) (*ytdlp.Listing, error) {
		mu.Lock()
		trace = append(trace, "read:"+raw)
		mu.Unlock()
		return listing(raw, raw+"-song"), nil
	}
	if err := e.Download(context.Background(), []string{"new"}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	want := []string{"done:old", "read:queued", "done:queued-song", "read:new", "done:new-song"}
	if len(trace) != len(want) {
		t.Fatalf("unexpected sequence: %v", trace)
	}
	for i := range want {
		if trace[i] != want[i] {
			t.Fatalf("backlog/FIFO order changed: %v", trace)
		}
	}
}

func TestSkipCompletedSourceUnlessExplicitSync(t *testing.T) {
	e := queueEngine(t, 1)
	source := e.m.Merge("playlist", "PLsaved", "Saved", "https://music.youtube.com/playlist?list=PLsaved", []manifest.Entry{{ID: "old"}})
	finishTrack(t, e, context.Background(), "old", func(Event) {})
	lastSync := source.LastSync
	var reads atomic.Int32
	e.listSource = func(context.Context, string, string) (*ytdlp.Listing, error) {
		reads.Add(1)
		l := listing("PLsaved", "old", "new")
		l.Type = "playlist"
		return l, nil
	}
	alias := "https://www.youtube.com/watch?v=old&list=PLsaved&si=share"
	if err := e.Download(context.Background(), []string{alias}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 0 || !source.LastSync.Equal(lastSync) {
		t.Fatal("normal addition re-read a completed playlist")
	}
	if err := e.Sync(context.Background(), []string{alias}, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if reads.Load() != 1 || e.m.Tracks["new"].Status != manifest.StatusDone {
		t.Fatal("explicit sync did not discover the new track")
	}
}

func TestQueueSurvivesCancellationAndReopen(t *testing.T) {
	e := queueEngine(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	e.listSource = func(context.Context, string, string) (*ytdlp.Listing, error) { return listing("first", "a1"), nil }
	e.trackWork = func(ctx context.Context, id string, emit func(Event)) bool { cancel(); <-ctx.Done(); return false }
	if err := e.Download(ctx, []string{"first", "second"}, func(Event) {}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel: %v", err)
	}
	persisted, err := manifest.Load(e.Root())
	if err != nil || len(persisted.Queue) != 1 || persisted.Queue[0].URL != "second" || persisted.Tracks["a1"].Status != manifest.StatusPending {
		t.Fatalf("lost pending work: %+v, %v", persisted, err)
	}
	reopened, err := Open(e.Root(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	reopened.spaceCheck = func(string, uint64) error { return nil }
	reopened.trackWork = func(ctx context.Context, id string, emit func(Event)) bool {
		return finishTrack(t, reopened, ctx, id, emit)
	}
	reopened.listSource = func(ctx context.Context, raw, cookies string) (*ytdlp.Listing, error) {
		if raw != "second" || reopened.m.Tracks["a1"].Status != manifest.StatusDone {
			t.Error("re-read first source or failed to prioritize saved tracks")
		}
		return listing("second", "b1"), nil
	}
	if err := reopened.Download(context.Background(), nil, func(Event) {}); err != nil {
		t.Fatal(err)
	}
	if reopened.HasPending() {
		t.Fatal("resume left completed work pending")
	}
}

func TestFailedListingStaysQueuedWithoutBlockingLaterAlbums(t *testing.T) {
	e := queueEngine(t, 1)
	e.listSource = func(ctx context.Context, raw, cookies string) (*ytdlp.Listing, error) {
		if raw == "bad" {
			return nil, errors.New("network timeout")
		}
		return listing("good", "good-song"), nil
	}
	if err := e.Download(context.Background(), []string{"bad", "good"}, func(Event) {}); err == nil {
		t.Fatal("listing failure was reported as success")
	}
	if len(e.m.Queue) != 1 || e.m.Queue[0].URL != "bad" || e.m.Tracks["good-song"].Status != manifest.StatusDone {
		t.Fatal("failed link was lost or blocked later work")
	}
	e.listSource = func(ctx context.Context, raw, cookies string) (*ytdlp.Listing, error) {
		if raw != "bad" {
			t.Error("re-read completed source")
		}
		return listing("bad", "fixed-song"), nil
	}
	if err := e.Download(context.Background(), nil, func(Event) {}); err != nil {
		t.Fatal(err)
	}
}

func TestAddDuringDownloadPersistsBehindExistingWork(t *testing.T) {
	e := queueEngine(t, 2)
	started, release := make(chan struct{}), make(chan struct{})
	var mu sync.Mutex
	var readOrder []string
	e.listSource = func(ctx context.Context, raw, cookies string) (*ytdlp.Listing, error) {
		mu.Lock()
		readOrder = append(readOrder, raw)
		mu.Unlock()
		return listing(raw, raw+"-song"), nil
	}
	e.trackWork = func(ctx context.Context, id string, emit func(Event)) bool {
		if id == "first-song" {
			close(started)
			select {
			case <-release:
			case <-ctx.Done():
				return false
			}
		}
		return finishTrack(t, e, ctx, id, emit)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- e.Download(ctx, []string{"first", "second"}, func(Event) {}) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("first album did not start")
	}
	if active, err := e.Add([]string{"third", "third"}); !active || err != nil {
		t.Fatalf("live Add: %v, %v", active, err)
	}
	persisted, err := manifest.Load(e.Root())
	if err != nil || len(persisted.Queue) != 2 || persisted.Queue[0].URL != "second" || persisted.Queue[1].URL != "third" {
		t.Fatalf("live queue not durable/FIFO: %+v, %v", persisted, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if len(readOrder) != 3 || readOrder[0] != "first" || readOrder[1] != "second" || readOrder[2] != "third" {
		t.Fatalf("live queue reordered: %v", readOrder)
	}
}
