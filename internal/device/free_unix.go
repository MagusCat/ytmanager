//go:build linux || darwin

package device

import "syscall"

func diskSpace(path string) (SpaceInfo, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return SpaceInfo{}, err
	}
	return SpaceInfo{Available: st.Bavail * uint64(st.Bsize), Total: st.Blocks * uint64(st.Bsize)}, nil
}
