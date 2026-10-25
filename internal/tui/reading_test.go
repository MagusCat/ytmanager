package tui

import (
	"strings"
	"testing"
	"time"

	"ytmanager/internal/engine"
)

func TestReadingActivityAnimatesFitsAndClears(t *testing.T) {
	m := scrollModel(t)
	m.run.busy = true
	m.apply(engine.Event{Kind: engine.EvReading, ID: "https://youtube.com/playlist?list=example"})
	before := m.readingActivity(80)
	updated, _ := m.Update(catTickMsg{})
	m = updated.(model)
	if before == m.readingActivity(80) {
		t.Fatal("waiting indicator did not animate")
	}
	for _, size := range [][2]int{{80, 24}, {40, 16}, {30, 12}} {
		m = sized(m, size[0], size[1])
		view := assertFits(t, m)
		if !strings.Contains(view, "Reading link") {
			t.Fatalf("reading state hidden at %v:\n%s", size, view)
		}
	}
	m.run.readingSince = time.Now().Add(-45 * time.Second)
	if !strings.Contains(m.readingActivity(80), "Still waiting") {
		t.Fatal("long wait not explained")
	}
	m.apply(engine.Event{Kind: engine.EvReadDone, ID: "different"})
	if m.run.readingURL == "" {
		t.Fatal("unrelated result cleared current read")
	}
	m.apply(engine.Event{Kind: engine.EvReadDone, ID: m.run.readingURL})
	if m.readingActivity(80) != "" {
		t.Fatal("finished read still shown")
	}
}
