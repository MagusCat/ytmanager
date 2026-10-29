package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExportM3U creates explicitly requested UTF-8 M3U copies beside the originals.
func (e *Engine) ExportM3U(ctx context.Context, emit func(Event)) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.running {
		return fmt.Errorf("wait for the current operation to finish")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := e.m.WritePlaylists(); err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(e.m.Root, "*.m3u8"))
	if err != nil {
		return err
	}
	for _, p := range files {
		if err := ctx.Err(); err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		dest := strings.TrimSuffix(p, ".m3u8") + ".m3u"
		const marker = "#EXTM3U\n# ytmanager compatibility export\n"
		previous, readErr := os.ReadFile(dest)
		if readErr == nil {
			if !strings.HasPrefix(string(previous), marker) {
				emit(Event{Kind: EvLog, Msg: "Kept custom playlist: " + filepath.Base(dest)})
				continue
			}
			if err := os.WriteFile(dest+".bak", previous, 0644); err != nil {
				return err
			}
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
		b = []byte(marker + strings.TrimPrefix(string(b), "#EXTM3U\n"))
		if err := os.WriteFile(dest, b, 0644); err != nil {
			return err
		}
		emit(Event{Kind: EvLog, Msg: "Exported: " + filepath.Base(dest)})
	}
	return nil
}
