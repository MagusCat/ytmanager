package engine

import (
	"os"
	"path/filepath"
	"testing"

	"ytmanager/internal/manifest"
)

func TestQueueCanRetryAfterSaveFailure(t *testing.T) {
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	root := e.m.Root
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, nil, 0600); err != nil {
		t.Fatal(err)
	}
	e.m.Root = blocked
	if _, err := e.Add([]string{"retry"}); err == nil {
		t.Fatal("expected queue save to fail")
	}
	if len(e.m.Queue) != 0 {
		t.Fatal("failed save left an apparently persisted request")
	}
	e.m.Root = root
	if _, err := e.Add([]string{"retry"}); err != nil {
		t.Fatal(err)
	}
	saved, err := manifest.Load(root)
	if err != nil || len(saved.Queue) != 1 {
		t.Fatalf("retry was not persisted: %+v, %v", saved, err)
	}
}

// Add persists even while a scheduler is finishing; it never silently loses URLs.
func TestAddAndIdleQueue(t *testing.T) {
	e, err := Open(t.TempDir(), Options{Jobs: 1})
	if err != nil {
		t.Fatal(err)
	}
	if active, err := e.Add([]string{"a"}); err != nil || active {
		t.Fatalf("idle Add: %v, %v", active, err)
	}
	e.running, e.accepting, e.more = true, true, make(chan struct{}, 1)
	if active, err := e.Add([]string{"b", "c", "b"}); err != nil || !active {
		t.Fatalf("running Add: %v, %v", active, err)
	}
	if len(e.m.Queue) != 3 {
		t.Fatalf("requests lost or duplicated: %+v", e.m.Queue)
	}
	if _, ok := e.nextRequest(map[string]bool{"a": true, "b": true, "c": true}); ok || e.accepting {
		t.Fatal("idle scheduler still accepts requests")
	}
	if active, err := e.Add([]string{"d"}); err != nil || active {
		t.Fatalf("closing Add: %v, %v", active, err)
	}
	persisted, err := manifest.Load(e.Root())
	if err != nil || len(persisted.Queue) != 4 {
		t.Fatalf("late request not durable: %+v, %v", persisted, err)
	}
}
