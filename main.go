package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"ytmanager/internal/engine"
	"ytmanager/internal/tui"
	"ytmanager/internal/ytdlp"
)

func main() {
	saved := engine.LoadOptions() // what the settings panel saved; flags override it
	format := flag.String("format", "", "opus | m4a | mp3 (saved in the destination's data.music)")
	jobs := flag.Int("jobs", saved.Jobs, "parallel downloads")
	cookies := flag.String("cookies", saved.Cookies, "browser to read cookies from (firefox, chrome, brave…) for private playlists or Liked Music")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `ytmanager — download playlists, albums and songs from YouTube / YT Music

  ytmanager [flags] [destination]             TUI
  ytmanager [flags] sync <destination>        re-sync everything saved in data.music
  ytmanager [flags] add <destination> <url>…  add sources and download them

`)
		flag.PrintDefaults()
	}
	flag.Parse()

	for _, bin := range []string{"yt-dlp", "ffmpeg"} {
		if _, err := exec.LookPath(bin); err != nil {
			die(fmt.Errorf("%s not found in PATH; install it with your package manager (see https://github.com/yt-dlp/yt-dlp#installation)", bin))
		}
	}
	if *format != "" && !ytdlp.ValidFormat(*format) {
		die(fmt.Errorf("invalid format %q (opus, m4a, mp3)", *format))
	}
	opt := engine.Options{Cookies: *cookies, Jobs: *jobs}

	args := flag.Args()
	if len(args) >= 2 && (args[0] == "sync" || args[0] == "add") {
		die(headless(args[0], args[1], args[2:], *format, opt))
		return
	}
	dest := ""
	if len(args) > 0 {
		dest = args[0]
	}
	die(tui.Run(dest, opt, *format))
}

func headless(cmd, dest string, urls []string, format string, opt engine.Options) error {
	eng, err := engine.Open(dest, opt)
	if err != nil {
		return err
	}
	if format != "" {
		if err := eng.SetFormat(format); err != nil {
			return err
		}
	}
	if cmd == "sync" {
		for _, s := range eng.Snapshot() {
			urls = append(urls, s.URL)
		}
	}
	if len(urls) == 0 {
		return errors.New("no URLs: use add <destination> <url>…")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return eng.Sync(ctx, urls, func(e engine.Event) {
		switch e.Kind {
		case engine.EvLog:
			fmt.Println(e.Msg)
		case engine.EvDone:
			fmt.Println("✓", e.Title, e.Msg)
		case engine.EvFailed:
			fmt.Println("✗", e.Title+":", e.Msg)
		case engine.EvUnavailable:
			fmt.Println("-", e.Title+": unavailable; set an alternative from the TUI (n)")
		}
	})
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
