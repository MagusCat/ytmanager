# Contributing

ytmanager is written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
drives `yt-dlp` and `ffmpeg` as external programs.

```sh
go build -o ytmanager .
go test ./...
```

## Layout

```
main.go          flags, headless commands, starts the TUI
internal/
├── tui/         screens: start, folder browser, sources, library, settings, unavailable, cat
├── engine/      sync queue and workers, pause, tags, moving, repair, scan, delete
├── ytdlp/       wraps yt-dlp and ffprobe: list, read metadata, download, name files
├── ytmusic/     YouTube Music song/video relationship
├── manifest/    data.music and .m3u8 playlists
├── lyrics/      LRCLIB and YT Music lyrics
└── device/      destinations: folders, USB drives, MTP phones, free space
```

For each song the engine reads its metadata, fixes the tags, looks up lyrics, lets yt-dlp
download and convert it in a temp folder, copies it to the destination and records it in
`data.music`.

## Adding features

| To add… | Change |
|---|---|
| A key | The screen's key `switch` in `internal/tui/`, its hint in `help.go`, and the keys table in the README. |
| A setting | A field in `Options` (`internal/engine/engine.go`), its row in `internal/tui/settings.go`, and a flag in `main.go` if needed. |
| A headless command | The command check in `main.go`, calling a function in `internal/engine/`. |
| Download or tagging logic | `internal/engine/` (`track.go`, `metadata.go`) or `internal/ytdlp/`. |
| A destination type | `internal/device/`, one file per OS. |
| A lyrics provider | `internal/lyrics/`, following `lrclib.go`. |

- Keep logic in `engine/`; `tui/` only displays state and sends actions.
- Everything saved per destination goes through `manifest/`.
- Anything that deletes or moves files asks for confirmation and leaves files intact when
  interrupted.
- New logic gets a `_test.go` next to it. Run `gofmt` and `go test ./...` before a pull request.

## Screenshots

Images live in `docs/screenshots/` (`banner.png`, `menu.png`, `download.png`, `config.png`).
Run `ytmanager` in a terminal of at least 100×30, capture only the terminal window and overwrite
the file to update it. Add new ones to the README's Screenshots section with descriptive alt
text. Keep GIFs (e.g. recorded with [vhs](https://github.com/charmbracelet/vhs)) under 5 MB.
