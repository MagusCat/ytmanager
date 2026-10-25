package engine

import (
	"errors"
	"testing"

	"ytmanager/internal/manifest"
)

// An unknown error marks a track unavailable after maxFails syncs; network errors never count.
func TestRecordFails(t *testing.T) {
	tr := &manifest.Track{}
	for range maxFails * 2 {
		record(tr, "/", "", "", errors.New("Unable to download webpage: timed out"))
	}
	if tr.Status != manifest.StatusFailed || tr.Fails != 0 {
		t.Fatalf("network errors: status %s fails %d", tr.Status, tr.Fails)
	}
	for i := 1; i <= maxFails; i++ {
		record(tr, "/", "", "", errors.New("Sign in to confirm your age"))
		if want := i == maxFails; (tr.Status == manifest.StatusUnavailable) != want {
			t.Fatalf("after %d failures status %s", i, tr.Status)
		}
	}
	record(tr, "/r", "/r/a.opus", "", nil)
	if tr.Status != manifest.StatusDone || tr.Fails != 0 || tr.Path != "a.opus" {
		t.Fatalf("success: %+v", tr)
	}
}
