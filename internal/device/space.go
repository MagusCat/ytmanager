package device

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type SpaceInfo struct {
	Available uint64
	Total     uint64
}

// SpaceReserve leaves room for manifests, playlists and filesystem metadata.
const SpaceReserve uint64 = 32 << 20

var ErrLowSpace = errors.New("not enough free space")

// Space distinguishes an unknown capacity from a full disk. Ask gvfs for phone
// storage rather than treating the computer's FUSE mount as phone capacity.
func Space(path string) (SpaceInfo, error) {
	for {
		if _, err := os.Stat(path); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return SpaceInfo{}, err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return SpaceInfo{}, os.ErrNotExist
		}
		path = parent
	}
	var info SpaceInfo
	var err error
	if IsMTP(path) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		out, runErr := exec.CommandContext(ctx, "gio", "info", "-f", "-a", "filesystem::free,filesystem::size", path).Output()
		if runErr != nil {
			return SpaceInfo{}, runErr
		}
		info, err = parseSpace(string(out))
	} else {
		info, err = diskSpace(path)
	}
	if err != nil {
		return SpaceInfo{}, err
	}
	if info.Total == 0 || info.Available > info.Total {
		return SpaceInfo{}, errors.New("storage capacity is unknown")
	}
	return info, nil
}

func parseSpace(out string) (SpaceInfo, error) {
	var info SpaceInfo
	var haveFree, haveTotal bool
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		for name, target := range map[string]*uint64{"filesystem::free:": &info.Available, "filesystem::size:": &info.Total} {
			if value, ok := strings.CutPrefix(line, name); ok {
				n, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
				if err != nil {
					return info, err
				}
				*target = n
				haveFree = haveFree || name == "filesystem::free:"
				haveTotal = haveTotal || name == "filesystem::size:"
			}
		}
	}
	if !haveFree || !haveTotal {
		return info, errors.New("phone did not report free space")
	}
	return info, nil
}

func Free(path string) uint64 {
	info, _ := Space(path)
	return info.Available
}

// RequireSpace skips unavailable measurements, but a known zero is full.
func RequireSpace(path string, needed uint64) error {
	info, err := Space(path)
	if err != nil {
		return nil
	}
	return checkSpace(info, needed)
}

func checkSpace(info SpaceInfo, needed uint64) error {
	if info.Available < SpaceReserve || needed > info.Available-SpaceReserve {
		return fmt.Errorf("%w: %.1f MB free, %.1f MB required plus 32 MB reserve", ErrLowSpace, float64(info.Available)/(1<<20), float64(needed)/(1<<20))
	}
	return nil
}

func IsSpaceError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrLowSpace) || errors.Is(err, syscall.ENOSPC) || errors.Is(err, syscall.EDQUOT) {
		return true
	}
	// External yt-dlp/ffmpeg processes report filesystem failures through stderr.
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "no space left on device") || strings.Contains(message, "disk full") || strings.Contains(message, "disk quota exceeded")
}
