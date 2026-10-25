package device

import (
	"errors"
	"net/url"
	"os"

	"golang.org/x/sys/windows"
)

// drives lists every lettered drive except C:, where ~/Music already lives.
func drives() []string {
	var out []string
	for l := 'D'; l <= 'Z'; l++ {
		p := string(l) + `:\`
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// Windows exposes MTP phones only through the shell, not as drive paths.
func phoneStorages() []string { return nil }

func MountPhones() {}

func resolveMTP(*url.URL) (string, error) {
	return "", errors.New("mtp:// is not supported on Windows: copy to a folder and sync it with your phone")
}

func phoneLabel(string) (string, string, bool) { return "", "", false }

func diskSpace(path string) (SpaceInfo, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return SpaceInfo{}, err
	}
	var avail, total uint64
	if err := windows.GetDiskFreeSpaceEx(p, &avail, &total, nil); err != nil {
		return SpaceInfo{}, err
	}
	return SpaceInfo{Available: avail, Total: total}, nil
}
