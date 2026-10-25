package ytdlp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"ytmanager/internal/device"
)

// ErrInvalidAudio identifies files that need to be downloaded again, rather than
// temporary device, permission or tool failures.
var ErrInvalidAudio = errors.New("invalid audio file")

// ErrMissingAudio distinguishes a missing library file from a failed temporary copy.
var ErrMissingAudio = errors.New("missing audio file")

// Tags reads format and stream tags, including Opus stream metadata. Phone files
// are staged locally because ffprobe may need seeking that MTP cannot provide.
func Tags(ctx context.Context, path string) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return nil, tagReadError(path, err)
	}
	if stat.Size() == 0 {
		return nil, fmt.Errorf("audio file is empty; sync (Enter) to restore it: %w", ErrInvalidAudio)
	}
	input := path
	if device.IsMTP(path) {
		local, cleanup, err := stageProbe(ctx, path)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, fmt.Errorf("cannot copy phone audio for tag reading; check the USB connection and free space in the computer's temporary folder: %w", err)
		}
		defer cleanup()
		input = local
	}
	out, err := exec.CommandContext(ctx, "ffprobe", "-v", "error", "-show_entries", "format_tags:stream_tags", "-of", "json", "-i", input).Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, probeError(input, err)
	}
	var r struct {
		Format  struct{ Tags map[string]string }   `json:"format"`
		Streams []struct{ Tags map[string]string } `json:"streams"`
	}
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("ffprobe returned invalid metadata: %w", err)
	}
	tags := map[string]string{}
	for _, stream := range append(r.Streams, r.Format) {
		for k, v := range stream.Tags {
			tags[strings.ToLower(k)] = v
		}
	}
	for alias, key := range map[string]string{"albumartist": "album_artist", "album artist": "album_artist", "tracknumber": "track", "track_number": "track"} {
		if tags[key] == "" && tags[alias] != "" {
			tags[key] = tags[alias]
		}
	}
	return tags, nil
}

func tagReadError(path string, err error) error {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return err
	case errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("file is missing; sync (Enter) to restore it: %w: %w", ErrMissingAudio, err)
	case device.IsMTP(path):
		return fmt.Errorf("cannot read phone file; unlock the phone, enable USB File transfer and retry: %w", err)
	case errors.Is(err, os.ErrPermission):
		return fmt.Errorf("cannot read audio; check the file's read permissions: %w", err)
	default:
		return fmt.Errorf("cannot read audio file: %w", err)
	}
}

func probeError(input string, err error) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("ffprobe is missing; install ffmpeg with ffprobe: %w", err)
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) || len(exit.Stderr) == 0 {
		return fmt.Errorf("cannot read audio tags: %w", err)
	}
	detail := strings.Join(strings.Fields(strings.ReplaceAll(string(exit.Stderr), input+": ", "")), " ")
	if strings.Contains(detail, "Invalid data found") || strings.Contains(detail, "End of file") {
		return fmt.Errorf("audio could not be decoded; sync (Enter) to restore it (%s): %w", detail, ErrInvalidAudio)
	}
	return fmt.Errorf("ffprobe could not read audio tags: %s: %w", detail, err)
}

func stageProbe(ctx context.Context, path string) (string, func(), error) {
	in, err := os.Open(path)
	if err != nil {
		return "", nil, err
	}
	defer in.Close()
	stat, err := in.Stat()
	if err != nil {
		return "", nil, err
	}
	if err := device.RequireSpace(os.TempDir(), uint64(stat.Size())); err != nil {
		return "", nil, err
	}
	out, err := os.CreateTemp("", "ytm-probe-*"+filepath.Ext(path))
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { os.Remove(out.Name()) }
	_, err = io.Copy(out, probeReader{ctx, in})
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		cleanup()
		return "", nil, err
	}
	return out.Name(), cleanup, nil
}

type probeReader struct {
	ctx context.Context
	io.Reader
}

func (r probeReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.Reader.Read(p)
}
