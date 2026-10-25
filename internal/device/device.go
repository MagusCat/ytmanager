// Package device finds destinations to download into: ~/Music, removable drives and, on Linux,
// phones mounted over MTP. Platform specifics live in device_<os>.go.
package device

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Mount struct {
	Path string
	Free uint64 // bytes available; 0 when unknown
}

// IsMTP reports whether a path belongs to a gvfs phone mount.
func IsMTP(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/gvfs/mtp:host=")
}

// List returns ~/Music followed by every removable drive and phone storage found.
func List() []Mount {
	var paths []string
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, "Music"))
	}
	paths = append(paths, drives()...)
	for _, p := range phoneStorages() {
		paths = append(paths, filepath.Join(p, "Music"))
	}
	out := make([]Mount, len(paths))
	for i, p := range paths {
		// Music may not exist yet (e.g. on a fresh phone): measure the storage that holds it.
		out[i] = Mount{Path: p, Free: Free(p)}
		if out[i].Free == 0 {
			out[i].Free = Free(filepath.Dir(p))
		}
	}
	return out
}

// Resolve turns what the user typed or pasted into a real path: expands ~/ and maps
// mtp:// URIs copied from a file manager to their mount point.
func Resolve(p string) (string, error) {
	if rest, ok := strings.CutPrefix(p, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest), nil
		}
	}
	if u, err := url.Parse(p); err == nil && u.Scheme == "mtp" {
		return resolveMTP(u)
	}
	return p, nil
}

// Label shortens a path for display: phones become "Phone <name> > <storage>", $HOME becomes ~.
func Label(p string) string {
	if name, rest, ok := phoneLabel(p); ok {
		return "Phone " + name + " › " + rest
	}
	if home, err := os.UserHomeDir(); err == nil && (p == home || strings.HasPrefix(p, home+string(filepath.Separator))) {
		return "~" + p[len(home):]
	}
	return p
}
