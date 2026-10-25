package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"

	"ytmanager/internal/manifest"
)

// Creators survive saving/reopening and distinguish identical source titles.
// Legacy manifests still display their original link without a metadata fetch.
func TestSourceCreators(t *testing.T) {
	root := t.TempDir()
	library, err := manifest.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, creator string }{{"one", "Artist One"}, {"two", "Artist Two"}, {"legacy", ""}} {
		s := library.Merge("album", item.id, "Greatest Hits", "https://music.youtube.com/playlist?list="+item.id, nil)
		s.Creator = item.creator
	}
	if err := library.Save(); err != nil {
		t.Fatal(err)
	}
	m := newModel()
	if err := m.openDest(root); err != nil {
		t.Fatal(err)
	}
	var view strings.Builder
	m.sourcesView(&view, 100)
	for _, want := range []string{"By Artist One", "By Artist Two", "https://music.youtube.com/playlist?list=legacy"} {
		if !strings.Contains(view.String(), want) {
			t.Errorf("sources view missing %q:\n%s", want, view.String())
		}
	}

	// Marking and syncing still use source identities, including equal titles.
	m = press(m, "down", " ")
	if urls := m.pickedURLs(m.eng.Snapshot()); len(urls) != 1 || !strings.HasSuffix(urls[0], "=two") {
		t.Fatalf("marked sources = %v", urls)
	}

	// Long, wide and multiline names stay on one bounded detail line.
	library.Sources[0].Creator = strings.Repeat("音楽", 30) + "\nother line"
	if err := library.Save(); err != nil {
		t.Fatal(err)
	}
	if err := m.openDest(root); err != nil {
		t.Fatal(err)
	}
	view.Reset()
	m.sourcesView(&view, 40)
	for _, line := range strings.Split(view.String(), "\n") {
		if strings.Contains(line, "By ") && lipgloss.Width(line) > 42 {
			t.Errorf("unbounded creator row: %q", line)
		}
		if strings.Contains(line, "音") && !strings.Contains(line, "…") {
			t.Errorf("long creator was not truncated: %q", line)
		}
	}
}
