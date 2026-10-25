package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Load reads data.music from root, falling back to the backup copy that Save keeps on MTP.
// A destination without either starts empty.
func Load(root string) (*Manifest, error) {
	p := filepath.Join(root, FileName)
	m, err := read(root, p)
	if m == nil {
		if bak, _ := read(root, p+".bak"); bak != nil {
			return bak, nil
		}
	}
	if err != nil {
		return nil, err
	}
	if m == nil {
		m = &Manifest{Version: 1, Format: "opus", Tracks: map[string]*Track{}, Root: root}
	}
	return m, nil
}

// read returns nil, nil when the file does not exist, and nil plus the error when it cannot be used.
func read(root, p string) (*Manifest, error) {
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	m := &Manifest{Root: root}
	if err := json.Unmarshal(b, m); err != nil {
		return nil, fmt.Errorf("%s is corrupt: %w", filepath.Base(p), err)
	}
	if m.Tracks == nil {
		m.Tracks = map[string]*Track{}
	}
	return m, nil
}

// Save never leaves a half-written manifest behind if the device is unplugged mid-write. On
// regular filesystems it writes a .tmp and renames it over the old file. MTP (gvfs) loses files
// on rename, so there it writes a full .bak first and then the real file; Load uses the .bak if
// the real file was cut short.
func (m *Manifest) Save() error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(m.Root, FileName)
	if strings.Contains(p, "/gvfs/mtp:host=") {
		if err := os.WriteFile(p+".bak", b, 0o644); err != nil {
			return err
		}
		return os.WriteFile(p, b, 0o644)
	}
	if err := os.WriteFile(p+".tmp", b, 0o644); err != nil {
		return err
	}
	return os.Rename(p+".tmp", p)
}
