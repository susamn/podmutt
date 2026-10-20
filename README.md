<p align="center">
  <img src="assets/logo.svg" width="160" alt="podmutt logo">
</p>

# podmutt

A modern, keyboard-driven podcast client (TUI) for Linux and macOS inspired by lazygit and mpdtui. Built with Go, Bubble Tea, SQLite, and mpv.

[![Release](https://img.shields.io/github/v/release/susamn/podmutt)](https://github.com/susamn/podmutt/releases)
[![Go Version](https://img.shields.io/badge/go-1.27%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

---

Bordered panels, single-key contextual actions, vim navigation, and live audio streaming through mpv — no mouse required.

## Screenshots

### Dashboard
Full 5-pane layout featuring Library, Episodes, Up next queue, Details card, and Now playing HUD.

![podmutt dashboard](assets/screenshot-dashboard.png)

### Discover & Preview
Directory browsing (Apple Top 100 / search) with customer star ratings and read-only episode previews without subscribing.

![podmutt discover](assets/screenshot-discover.png)

### Compact Mode
Responsive layout adapting automatically when terminal dimensions shrink.

![podmutt compact mode](assets/screenshot-compact.png)

---

## Features

- **Local-First & Offline**: Instant cached library browsing; feeds refresh periodically in the background without blocking the UI.
- **Top Charts & Global Discovery**: Browse Apple Podcasts Top 100 charts (auto-localized to your country, e.g. US, Germany, UK, France) with customer star ratings.
- **Stream Without Subscribing**: Preview show notes and stream/enqueue episodes directly from RSS without polluting your subscription catalog.
- **Fixed Player HUD**: Real-time progress bar, responsive layout, and dedicated ±15-second seeking hotkeys (`,`/`.`).
- **Queue Management**: Immediate queue deletion (`d`) with single-step session undo (`u`).
- **Podcast Artwork**: Crisp cover art via Kitty graphics protocol with automatic image-derived ASCII fallback.
- **Live Theming**: Color presets (Catppuccin, Solarized Dark, Cyberpunk) with live `SIGUSR1` reloading.
- **Desktop Media Integration**: Native MPRIS D-Bus support on Linux for desktop media keys.

---

## Install

### Homebrew (macOS and Linux)

```sh
brew tap susamn/tap
brew install --cask podmutt
```

Runtime playback streams audio through `mpv`:
- macOS: `brew install mpv`
- Ubuntu/Debian: `sudo apt install mpv`
- Arch Linux: `sudo pacman -S mpv`
- Fedora: `sudo dnf install mpv`

### Pre-built Binaries

Download pre-compiled standalone archives for Linux and macOS (`amd64` and `arm64`) from [GitHub Releases](https://github.com/susamn/podmutt/releases).

### Build from Source

Requires Go 1.27+ and Make:

```sh
git clone https://github.com/susamn/podmutt.git
cd podmutt
make build        # bin/podmutt
```

---

## Keybindings (Vim-Style)

### Navigation

| Key | Action |
| :--- | :--- |
| `Tab` / `Shift+Tab` | Cycle main panels (**Podcasts → Episodes → Queue**) |
| `1`–`5` | Directly focus pane (`1` Podcasts, `2` Episodes, `3` Queue, `4` Details, `5` Player) |
| `j` / `k` or `↑` / `↓` | Move selection up / down |
| `h` / `l` or `←` / `→` | Scroll content horizontally on narrow screens |
| `g` / `G` | Jump to first / last item |
| `Ctrl+d` / `Ctrl+u` | Scroll half-page down / up |
| `Esc` | Clear filter / return back from Details |

### Playback & Transport

| Key | Action |
| :--- | :--- |
| `Enter` | Open show / Play selected episode |
| `Space` | Pause / Resume current playback from any pane |
| `s` | Stop current playback |
| `n` | Skip to next episode in queue |
| `,` / `.` | Seek backward / forward 15 seconds |

### Library & Queue

| Key | Action |
| :--- | :--- |
| `a` | Enqueue selected episode to Up Next |
| `A` | Enqueue all visible episodes in current list |
| `d` / `Delete` | Remove pending episode from Up Next |
| `u` | Undo last removal (in Queue) / Toggle subscription (in Podcasts) |
| `D` | Clear all pending queue items (with confirmation) |
| `o` | Cycle sorting (newest, oldest, title) |
| `c` | Toggle episode completed mark |
| `R` | Manually refresh feeds or directory |

### Discovery & Search

| Key | Action |
| :--- | :--- |
| `b` | Toggle between **My Library** and **Discover** (Top 100) |
| `f` | Find podcasts by name or paste an RSS/website URL |
| `U` | Open direct RSS/website preview dialog |
| `t` | Jump to Top 100 chart in Discover |
| `N` | Add podcast feed URL directly |
| `/` | Filter current list |

### System & Settings

| Key | Action |
| :--- | :--- |
| `e` | Settings (Theme picker, Country switcher) |
| `?` | Open keybinding cheat sheet |
| `q` / `Ctrl+C` | Quit |

---

## Configuration

The configuration file is located at:
- **Linux**: `~/.config/podmutt/config.toml`
- **macOS**: `~/Library/Application Support/podmutt/config.toml`

An absent file uses these defaults:

```toml
refresh_minutes = 30
mpv_path = "mpv"
country = "us"             # Auto-detected from system locale (e.g. "de", "gb", "fr")
theme = "colors"           # "colors", "solarized-dark", "catppuccin", etc.
icons = "unicode"          # "unicode" (default), "nerd" (Nerd Font), or "plain"
player_decoration = false  # Optional hairline visualizer in spacious layouts
```

---

## Running-Instance CLI Commands

`podmutt` supports CLI commands that contact the running instance over a private UNIX socket:

```sh
podmutt play
podmutt pause
podmutt stop
podmutt next
podmutt status   # outputs JSON status of running instance
```

---

## Development

```sh
make check       # formatting, module consistency, vet, tests, >=90% coverage
make race        # native race-detector tests
make mpv-test    # real-mpv lifecycle/playback tests, null audio (requires mpv)
make mpris-test  # private D-Bus protocol tests (requires dbus-daemon)
make build       # build bin/podmutt
make dist        # cross-build Linux/macOS archives in dist/
```

---

## License

Licensed under the [MIT License](LICENSE).
