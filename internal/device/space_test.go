package device

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
	"testing"
)

func TestSpaceAndMissingSubfolder(t *testing.T) {
	root := t.TempDir()
	space, err := Space(root)
	if err != nil || space.Total == 0 || space.Available > space.Total {
		t.Fatalf("storage: %+v, %v", space, err)
	}
	child, err := Space(filepath.Join(root, "not-created", "yet"))
	if err != nil || child.Total != space.Total {
		t.Fatalf("subfolder: %+v, %v", child, err)
	}
}

func TestPhoneSpaceDistinguishesZeroAndUnknown(t *testing.T) {
	for _, free := range []uint64{0, 123456} {
		info, err := parseSpace(fmt.Sprintf("attributes:\n  filesystem::size: 9999999\n  filesystem::free: %d\n", free))
		if err != nil || info.Available != free || info.Total != 9999999 {
			t.Fatalf("phone space: %+v, %v", info, err)
		}
	}
	if _, err := parseSpace("filesystem::size: 9999999"); err == nil {
		t.Fatal("missing free-space attribute was interpreted as full storage")
	}
}

func TestSpaceReserve(t *testing.T) {
	for _, available := range []uint64{0, SpaceReserve - 1, SpaceReserve + 1023} {
		if err := checkSpace(SpaceInfo{Available: available, Total: 1 << 30}, 1024); !errors.Is(err, ErrLowSpace) {
			t.Fatalf("%d available should reject the copy: %v", available, err)
		}
	}
	if err := checkSpace(SpaceInfo{Available: SpaceReserve + 1024, Total: 1 << 30}, 1024); err != nil {
		t.Fatal(err)
	}
	for _, err := range []error{ErrLowSpace, syscall.ENOSPC, syscall.EDQUOT, errors.New("ffmpeg: No space left on device")} {
		if !IsSpaceError(fmt.Errorf("copy failed: %w", err)) {
			t.Fatalf("storage error not recognized: %v", err)
		}
	}
	if IsSpaceError(errors.New("Video unavailable")) || IsSpaceError(nil) {
		t.Fatal("unrelated error classified as storage failure")
	}
}
