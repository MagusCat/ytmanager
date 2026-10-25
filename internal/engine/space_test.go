package engine

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"ytmanager/internal/device"
	"ytmanager/internal/manifest"
)

func TestStorageFailuresStayPending(t *testing.T) {
	for _, err := range []error{device.ErrLowSpace, syscall.ENOSPC, errors.New("ffmpeg: No space left on device")} {
		track := &manifest.Track{Path: "old.opus", Status: manifest.StatusDone}
		for range maxFails + 1 {
			record(track, "/root", "", "", err)
		}
		if track.Status != manifest.StatusPending || track.Fails != 0 || track.Path != "old.opus" {
			t.Fatalf("space failure lost a downloaded file or marked it unavailable: %+v", track)
		}
	}
}

func TestPlanChecksDestinationAndTemporaryStorage(t *testing.T) {
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	e.m.Merge("playlist", "source", "Source", "unused", []manifest.Entry{{ID: "song", Duration: 180}})
	var checked []string
	e.spaceCheck = func(path string, need uint64) error {
		checked = append(checked, path)
		if path != e.Root() {
			return device.ErrLowSpace
		}
		if need < 180*40000 {
			t.Error("estimated destination space missing")
		}
		return nil
	}
	ids, err := e.plan([]string{"source"}, map[string]bool{}, func(Event) {})
	if len(ids) != 0 || !errors.Is(err, device.ErrLowSpace) || len(checked) != 2 {
		t.Fatalf("plan ignored temporary storage: %v, %v, %v", ids, err, checked)
	}
}

func TestRunningOperationDetectsFallingSpace(t *testing.T) {
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	var full atomic.Bool
	e.spaceCheck = func(string, uint64) error {
		if full.Load() {
			return device.ErrLowSpace
		}
		return nil
	}
	parent, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	ctx, cancel := e.watchSpace(parent)
	defer cancel(nil)
	full.Store(true)
	<-ctx.Done()
	if !errors.Is(context.Cause(ctx), device.ErrLowSpace) {
		t.Fatalf("running operation did not stop for low space: %v", context.Cause(ctx))
	}
}

func TestRearrangeRefusesLowSpaceWithoutMovingFiles(t *testing.T) {
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	track := &manifest.Track{Path: filepath.Join("Artist", "Song.opus"), Status: manifest.StatusDone}
	e.m.Tracks["song"] = track
	e.spaceCheck = func(string, uint64) error { return device.ErrLowSpace }
	err = e.Rearrange(context.Background(), func(Event) { t.Error("work started with low space") })
	if !errors.Is(err, device.ErrLowSpace) || track.Status != manifest.StatusDone || track.Path != filepath.Join("Artist", "Song.opus") {
		t.Fatalf("rearrange changed existing file state: %+v, %v", track, err)
	}
}

func TestSyncStopsWhenSpaceDropsDuringDownload(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake yt-dlp uses a POSIX shell")
	}
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not installed")
	}
	bin := t.TempDir()
	script := "#!/bin/sh\ncase \" $* \" in\n*' --flat-playlist '*) printf '%s\\n' '{\"_type\":\"playlist\",\"id\":\"source\",\"title\":\"Source\",\"entries\":[{\"id\":\"song\",\"title\":\"Song\",\"duration\":180}]}' ;;\n*) exec " + strconv.Quote(sleep) + " 10 ;;\nesac\n"
	if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	var checks atomic.Int32
	e.spaceCheck = func(string, uint64) error {
		if checks.Add(1) > 2 { // planning succeeds; the next runtime check fails
			return device.ErrLowSpace
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = e.Sync(ctx, []string{"https://example.com/playlist?list=source"}, func(Event) {})
	if !errors.Is(err, device.ErrLowSpace) {
		t.Fatalf("sync did not return its storage stop reason: %v", err)
	}
	library, err := manifest.Load(e.Root())
	if err != nil {
		t.Fatal(err)
	}
	if track := library.Tracks["song"]; track == nil || track.Status != manifest.StatusPending || track.Fails != 0 {
		t.Fatalf("interrupted download was not kept pending: %+v", track)
	}
}
