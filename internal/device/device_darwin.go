package device

import (
	"errors"
	"net/url"
	"os"
	"path/filepath"
)

// drives lists /Volumes, skipping the startup disk (a symlink to /).
func drives() []string {
	entries, _ := os.ReadDir("/Volumes")
	var out []string
	for _, e := range entries {
		if e.Type()&os.ModeSymlink == 0 {
			out = append(out, filepath.Join("/Volumes", e.Name()))
		}
	}
	return out
}

// macOS does not mount MTP phones as folders.
func phoneStorages() []string { return nil }

func MountPhones() {}

func resolveMTP(*url.URL) (string, error) {
	return "", errors.New("mtp:// is not supported on macOS: copy to a folder and sync it with your phone")
}

func phoneLabel(string) (string, string, bool) { return "", "", false }
