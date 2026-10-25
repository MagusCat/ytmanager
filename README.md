# ytmanager

A terminal app that keeps a music library downloaded from YouTube and YT Music: playlists, albums,
mixes and single songs. It saves them to a folder, a USB drive or straight to an Android phone,
with clean tags, cover art and lyrics, so they look right in any music player.

It remembers what each destination holds, so syncing again only downloads what is new.

## Features

- **Playlists, albums, mixes and songs** from YouTube or YT Music. Paste a link and it starts.
- **Recognizable sources**: each source shows its artist, channel or playlist creator below the
  title. Older sources show their link until the next sync fills in the creator.
- **Incremental sync**: a `data.music` file in each destination tracks every song, so nothing is
  downloaded twice and deleted files come back on the next sync.
- **Persistent download queue**: saved unfinished tracks are resumed first. Sources are read and
  downloaded one album or playlist at a time by default; optional parallel albums share the song worker limit.
  Unread links survive closing the app. Adding a completed source skips its listing; an explicit
  sync refreshes it to discover new tracks.
- **Downloads straight to an Android phone** over MTP (Linux). Phones are detected and mounted
  automatically, and the sync resumes by itself if the cable is unplugged by accident.
- **Tags that music apps understand**: one album artist per album, the release year, track numbers,
  square cover art, and clean titles ("Artist - Song (Official Video)" becomes "Song").
- **Lyrics**: synced lyrics from [LRCLIB](https://lrclib.net) (saved as a `.lrc` file and embedded),
  falling back to the plain lyrics from YT Music.
- **`.m3u8` playlists** for every playlist and album, with relative paths that work on phones and
  car stereos.
- **Private playlists and Liked Music**: reads the YouTube cookies from your browser.
- **Unavailable songs** are marked and skipped. You can give them an alternative (a song, album or
  playlist link) and ytmanager picks the closest match.
- **Pause, resume, cancel**, and add more links while a download is running.
- **Reorganize anytime**: change the folder layout and move what is already downloaded, without
  downloading it again.
- A cat that keeps you company while it downloads.

## Requirements

| Tool | Why |
|------|-----|
| [yt-dlp](https://github.com/yt-dlp/yt-dlp#installation) | Lists and downloads from YouTube. Keep it up to date: YouTube changes often. |
| [ffmpeg](https://ffmpeg.org/download.html) (with `ffprobe`) | Converts the audio and writes tags and covers. |
| [Go](https://go.dev/dl/) 1.24 or newer | Only to build ytmanager. |
| gvfs (`gio`) | Only to download to an Android phone on Linux. Most desktops already have it. |

```sh
# Arch / CachyOS
sudo pacman -S yt-dlp ffmpeg go
# Debian / Ubuntu (the packaged yt-dlp is often outdated; prefer pipx)
sudo apt install ffmpeg golang && pipx install yt-dlp
# macOS
brew install yt-dlp ffmpeg go
```

## Install

```sh
git clone https://github.com/MagusCat/ytmanager.git
cd ytmanager
go build -o ytmanager .
sudo mv ytmanager /usr/local/bin/   # optional
```

## Usage

### Interactive (TUI)

```sh
ytmanager                 # pick a destination from the list
ytmanager ~/Music         # open a folder directly
```

1. **Pick where the music goes.** The first screen lists your music folder, USB drives and
   connected phones. Press `enter` to browse its folders, `n` to create one, `u` to use the folder
   you are in.
2. **Paste a link** (`ctrl+shift+v`). The download starts right away. Paste more while it runs and
   they join the queue.
3. **Next time**, open the same destination and press `enter` to continue the saved queue and
   unfinished downloads without reading completed playlists again. Press `s` to explicitly
   synchronize your marked sources (or all sources when none is marked) and find new tracks.

### Keys

| Screen | Keys |
|--------|------|
| Destination | `↑↓` move · `enter` browse folders · `e` type a path · `r` refresh · `q` quit |
| Folder browser | `→` open · `←` up · `n` new folder · `u` use this folder · `esc` back |
| Library | paste a link · `a` add links · `enter` resume/download · `s` sync · `space` mark sources · `d` remove a source · `p` pause/resume · `x` cancel · `n` unavailable songs · `o` settings · `c` change destination · `q` quit |
| Unavailable songs | paste or `enter` an alternative link · `r` retry · `esc` back |
| Settings | `↑↓` move · `←→` change · `enter` run an action · `esc` back |

Lists scroll to keep the selected row visible; `home` / `end` jump to the first / last source.
Logs have their own area and retain up to 500 messages. Use `PgUp` / `PgDn` to scroll them,
`Ctrl+Home` for the oldest retained message, and `Ctrl+End` to follow new messages again.
The mouse wheel scrolls the list or logs under the pointer. Both areas adapt to terminal resizing.

Picked the wrong folder? Press `esc`, `backspace` or `c` in the library to reopen the folder
browser. Use `←` to go up, `→` to enter a subfolder, `u` to choose it, or `e` to enter a path.
Press `esc` to return to the current library without changing it; `d` shows devices again.
Settings also include *Choose destination folder*. Finish or cancel an active operation before
switching destinations.

The library shows free space and capacity, refreshed every two seconds. During downloads and
reorganization, ytmanager checks the destination and local temporary storage and stops when
space drops below a 32 MB reserve. Copies also check room for the full file before writing.
Completed files stay in the library; free up space and sync to resume pending downloads.
Storage failures do not mark songs unavailable. If a device cannot report its capacity, the
indicator says so and actual write errors are still handled.

### Headless (scripts, cron)

```sh
ytmanager add  ~/Music https://music.youtube.com/playlist?list=...   # add sources and download them
ytmanager sync ~/Music                                               # re-sync everything in data.music
ytmanager resume ~/Music                                             # continue saved work without refreshing listings
```

Queued tracks already known to `data.music` run before any new source is read, including during
an explicit sync. New links join the end of the saved queue. A source that fails to list stays
queued for the next resume while later sources can continue. Each track is attempted once per
run (with the existing download retry); shared tracks are not queued twice.

Flags, usable in both modes:

| Flag | Default | |
|------|---------|---|
| `-format` | `opus` | `opus`, `m4a` or `mp3`. Saved per destination. |
| `-jobs` | `3` | Total parallel song downloads. |
| `-albums` | `1` | Simultaneous albums/playlists (1–4). |
| `-cookies` | none | Browser to read cookies from: `firefox`, `chrome`, `chromium`, `brave`, `edge`, `vivaldi`, `opera`. |

Destinations can also be given as `mtp://<phone>/<storage>/Music`.

## Settings

Press `o` in the library. Everything except the format is saved in
`~/.config/ytmanager/config.json` and applies to every destination.

| Setting | Options |
|---------|---------|
| Format | `opus` (smallest, best quality), `m4a` (Apple devices), `mp3` (old car stereos) |
| Parallel tracks | 1 to 8 total across all albums |
| Parallel albums | 1 to 4 sources |
| Browser cookies | Needed for private playlists and Liked Music. Log in to YouTube in that browser first. |
| Artists | `feat. in title`: "Song (feat. B)" by "A" (default) · `main only` · `all in artist`: "A, B" |
| Folders | `Artist/Album/01 - Song` (default) · `Artist/Song` · `Artist - Song` with no folders |
| Video titles | `clean` (default) or `as uploaded` |
| Move files to new folders | Applies the Folders setting to what is already downloaded, without downloading. |
| Download everything again | Applies every setting, including format, artists and titles. Each old file is replaced as its new one arrives. |
| Choose destination folder | Reopens the folder browser; `esc` returns to the current library. |

## What a destination looks like

```
Music/
├── data.music                 # what ytmanager knows about this destination
├── Liked Music.m3u8           # one playlist file per source
└── Bad Bunny/
    └── Un Verano Sin Ti/
        ├── 01 - Moscow Mule.opus
        └── 01 - Moscow Mule.lrc
```

Songs that come from singles or playlists with no album go to `Artist/Singles/`.

## Tips and troubleshooting

- **Liked Music (`list=LM`) shows 0 tracks**: it is private. Set *Browser cookies* in settings to a
  browser where you are logged in to YouTube.
- **Downloads fail suddenly**: update yt-dlp (`yt-dlp -U`, or update it with your package manager).
- **The phone does not appear**: unlock it and choose *File transfer* in the USB notification, then
  press `r`.
- **A song is unavailable**: open `n`, paste a link to another upload of the same song, or to the
  album that contains it.
- **Copying to the phone is slow**: MTP is slow by nature. Downloading to a folder and copying it
  over in one go is faster for big libraries.
- **Moving downloaded files fails**: use *Move files to new folders* in settings to retry. Phone
  files are read through a temporary local copy, one at a time; keep the phone unlocked with
  *File transfer* enabled and enough free space on the computer for one song. The error now
  includes the reason reported by `ffprobe`.
- **A file is missing, empty or cannot be decoded**: moving leaves existing audio untouched and
  marks that track pending. Press `enter` in the library to restore those tracks, then retry
  *Move files to new folders*. Other downloaded tracks are kept.
- **`ffprobe` is missing**: install the `ffmpeg` package with `ffprobe` and restart ytmanager.

## Platforms

| | Folders and USB drives | Android phone (MTP) |
|---|---|---|
| Linux | yes | yes, through gvfs |
| macOS | yes | no: download to a folder and copy it over |
| Windows | yes | no: download to a folder and copy it over |

## How it works

ytmanager is a Go program built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
It drives `yt-dlp` and `ffmpeg` as external programs.

```
main.go               flags, headless sync/add, starts the TUI
internal/
├── tui/              screens: destinations, folder browser, library, settings, unavailable songs, cat
├── engine/           the sync: download queue and workers, pause, tag normalisation, moving files
├── ytdlp/            wraps yt-dlp and ffprobe: list links, read metadata, download, name files
├── manifest/         data.music and the .m3u8 playlists
├── lyrics/           LRCLIB and YT Music lyrics
└── device/           destinations: music folder, USB drives, MTP phones, free space
```

For each song, the engine:

1. reads its full metadata with yt-dlp;
2. fixes the tags (album artist, year, album and track number, title);
3. looks up the lyrics;
4. hands everything to yt-dlp, which downloads, converts and tags the file in a temporary folder;
5. copies the result to the destination, which also works over MTP;
6. records the song in `data.music`, so the next sync skips it.

Run the tests with:

```sh
go test ./...
```

## Disclaimer

Download only content you have the right to download, and respect YouTube's Terms of Service and
the copyright laws of your country. This project is not affiliated with YouTube or Google.

### Parallel albums

In settings (`o`), choose **Parallel albums** (1–4). **Parallel tracks** is the total
worker limit shared by all active albums, so use at least 2 tracks to download
songs from different albums simultaneously. The default remains one album at a
time. Saved pending downloads still finish before new links are read.

Active sources show `downloading`; the activity area names the albums and each
track's source. For the CLI: `ytmanager -albums 2 -jobs 4 resume <destination>`.

### Download errors

`exit status 1` only means that an external program failed. The app now includes
its diagnostic output and identifies whether it failed while reading song information
or downloading/processing audio. Recognized errors include a suggested next step.
Unknown errors stay retryable; press Enter to resume after fixing the cause.
Songs previously marked unavailable can be retried from `n` with `r`.
