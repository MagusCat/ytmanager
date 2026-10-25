package ytdlp

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestFailedProcessDiagnostics(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell fixture")
	}
	for _, tc := range []struct{ name, script, want string }{
		{"plain", "echo 'ffmpeg conversion failed: invalid input' >&2\nexit 1", "invalid input"},
		{"indented", "echo '  ERROR: HTTP Error 429: Too Many Requests' >&2\nexit 1", "Wait, then retry"},
		{"empty", "exit 1", "did not provide a diagnostic"},
		{"large", "head -c 1100000 /dev/zero | tr '\\000' x\nprintf '\\nERROR: conversion failed\\n'\nexit 1", "yt-dlp:"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bin := t.TempDir()
			if err := os.WriteFile(filepath.Join(bin, "yt-dlp"), []byte("#!/bin/sh\n"+tc.script+"\n"), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_, err := run(ctx, t.TempDir(), "unused.json", "opus", "", "", func(float64) {})
			var exit *exec.ExitError
			if err == nil || !strings.Contains(err.Error(), tc.want) || !errors.As(err, &exit) {
				t.Fatalf("diagnostic lost: %v", err)
			}
			if tc.name != "large" {
				_, err = output(ctx, "-J", "unused")
				if err == nil || !strings.Contains(err.Error(), tc.want) || !errors.As(err, &exit) {
					t.Fatalf("metadata diagnostic lost: %v", err)
				}
			}
		})
	}
}

func TestDiagnosticBoundedAndPrefersError(t *testing.T) {
	var d diagnostic
	d.add("  ERROR: actual cause")
	for range 100 {
		d.add(strings.Repeat("x", 3000))
	}
	if len(d.lines) != 4 || d.detail() != "actual cause" {
		t.Fatalf("unexpected diagnostic: %d %s", len(d.lines), d.detail())
	}
}
