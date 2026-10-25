package ytdlp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"ytmanager/internal/device"
)

var audioArgs = map[string][]string{
	"opus": {"--audio-format", "opus"},
	"m4a":  {"--audio-format", "m4a"},
	"mp3":  {"--audio-format", "mp3", "--audio-quality", "0"},
}

func ValidFormat(f string) bool { _, ok := audioArgs[f]; return ok }

// Download fetches the audio of video id, described by an enriched info-json, into the folders
// layout and returns its final path under root.
//
// yt-dlp runs in a local staging folder and the result is copied to root: on MTP (gvfs) yt-dlp
// cannot move files (no utime) and --windows-filenames would rewrite the ':' in gvfs/mtp:host=….
// The folder is named after the video and kept when ctx is cancelled, so after a pause yt-dlp
// continues the .part file instead of starting over.
func Download(ctx context.Context, id, infoJSON, root, format, folders, cookies string, progress func(float64)) (string, error) {
	stage := filepath.Join(os.TempDir(), "ytmanager", id)
	if err := os.MkdirAll(stage, 0o755); err != nil {
		return "", err
	}
	file, err := run(ctx, stage, infoJSON, format, folders, cookies, progress)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		if !device.IsSpaceError(err) {
			os.RemoveAll(stage)
		}
		return "", err
	}
	rel, err := filepath.Rel(stage, file)
	if err != nil {
		os.RemoveAll(stage)
		return "", err
	}
	dst := filepath.Join(root, rel)
	err = CopyFile(file, dst)
	if !device.IsSpaceError(err) {
		os.RemoveAll(stage)
	}
	return dst, err
}

// run executes yt-dlp in stage and returns the absolute path of the file it produced.
func run(ctx context.Context, stage, infoJSON, format, folders, cookies string, progress func(float64)) (string, error) {
	args := append(base(cookies),
		"--load-info-json", infoJSON,
		"-f", "bestaudio/best", "-x",
		"--embed-metadata", "--embed-thumbnail", "--convert-thumbnails", "jpg",
		// square cover: center-crop the 16:9 thumbnail
		"--ppa", `ThumbnailsConvertor+FFmpeg_o:-c:v mjpeg -vf crop="'if(gt(ih,iw),iw,ih)':'if(gt(iw,ih),ih,iw)'"`,
		"-P", ".",
		"-o", layouts[folders],
		"--print", "after_move:FILE %(filepath)s",
		"--progress", "--newline",
		"--progress-template", "download:PROG %(progress.downloaded_bytes)s %(progress.total_bytes,progress.total_bytes_estimate)s",
	)
	args = append(append(args, naming...), audioArgs[format]...)

	pr, pw := io.Pipe()
	defer pr.Close()
	defer pw.Close()
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	cmd.Dir = stage
	cmd.Stdout, cmd.Stderr = pw, pw
	if err := cmd.Start(); err != nil {
		return "", processError(err, "could not start yt-dlp")
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait(); pw.Close() }()

	var file string
	var diagnostic diagnostic
	sc := bufio.NewScanner(pr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		diagnostic.add(line)
		switch {
		case strings.HasPrefix(line, "PROG "):
			if f := strings.Fields(line); len(f) == 3 {
				got, err1 := strconv.ParseFloat(f[1], 64)
				total, err2 := strconv.ParseFloat(f[2], 64)
				if err1 == nil && err2 == nil && total > 0 {
					progress(got / total)
				}
			}
		case strings.HasPrefix(line, "FILE "):
			file = strings.TrimPrefix(line, "FILE ")
		}
	}
	scanErr := sc.Err()
	if scanErr != nil {
		_, _ = io.Copy(io.Discard, pr)
	} // let the child finish even after an oversized line
	err := <-done
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		return "", processError(err, diagnostic.detail())
	}
	if scanErr != nil {
		return "", fmt.Errorf("could not read yt-dlp output: %w", scanErr)
	}
	if file == "" {
		return "", fmt.Errorf("yt-dlp did not report the final file")
	}
	if !filepath.IsAbs(file) {
		file = filepath.Join(stage, file)
	}
	return file, nil
}
