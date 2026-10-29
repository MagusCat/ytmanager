package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestScanActualFilesAndIdenticalCopies(t *testing.T) {
	root := t.TempDir()
	e, err := Open(root, Options{})
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "a.opus"), []byte("same bytes"), 0600)
	os.WriteFile(filepath.Join(root, "b.opus"), []byte("same bytes"), 0600)
	os.WriteFile(filepath.Join(root, "notes.txt"), []byte("same bytes"), 0600)
	t.Setenv("PATH", t.TempDir())
	rows, err := e.ScanLibrary(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d", len(rows))
	}
	for _, r := range rows {
		if !r.Unmanaged || !r.Duplicate || r.Size != 10 {
			t.Fatalf("bad row: %+v", r)
		}
	}
	if len(e.m.Tracks) != 0 {
		t.Fatal("scan modified manifest")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = e.ScanLibrary(ctx); err == nil {
		t.Fatal("ignored cancel")
	}
}
