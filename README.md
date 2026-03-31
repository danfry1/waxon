# waxon

A vim-modal Spotify client for the terminal, built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea).

Browse your library, control playback, and navigate playlists without leaving the terminal -- all with vim-style keybindings.

## Features

- Vim-style navigation (hjkl, gg/G, Ctrl+u/d, g-prefix motions)
- Two-pane layout with sidebar (library/queue) and tracklist
- Album art rendered in the terminal using Unicode half-block characters
- Full-screen Now Playing view with gradient backgrounds
- Vinyl record spinning easter egg (press `V` in Now Playing)
- Fuzzy filter and Spotify search
- Command mode for volume, shuffle, repeat, and more
- Context actions menu (go to artist/album, add to queue, copy URI)
- Spotify Connect device switching
- Browser-like back navigation history
- Lazy pagination for large playlists
- PKCE OAuth flow (no client secret required)

## Requirements

- Spotify Premium account
- A terminal with true color support (recommended)

## Installation

**Homebrew:**

```
brew install danfry1/tap/waxon
```

**Go:**

```
go install github.com/danfry1/waxon@latest
```

**Binary:** download from the [Releases](https://github.com/danfry1/waxon/releases) page.

## Quick Start

1. Run the setup wizard to authenticate with Spotify:

   ```
   waxon auth
   ```

2. Launch the TUI:

   ```
   waxon
   ```

## Keybindings

### Navigation

| Key              | Action              |
|------------------|---------------------|
| `j` / `k`        | Move down / up      |
| `gg`             | Go to top           |
| `G`              | Go to bottom        |
| `Ctrl+u` / `Ctrl+d` | Half page up / down |

### Panes

| Key              | Action              |
|------------------|---------------------|
| `h` / `l`        | Focus left / right pane |
| `Tab`            | Cycle pane          |
| `1` / `2`        | Library / queue section |

### Go-to (g prefix)

| Key   | Action                      |
|-------|-----------------------------|
| `gl`  | Go to library               |
| `gq`  | Go to queue                 |
| `gc`  | Jump to currently playing track |
| `gr`  | Recently played             |

### Playback

| Key              | Action              |
|------------------|---------------------|
| `Space`          | Play / pause        |
| `Enter`          | Play selected       |
| `n` / `p`        | Next / previous track |
| `[` / `]`        | Seek -5s / +5s      |

### Actions

| Key   | Action              |
|-------|---------------------|
| `o`   | Context actions menu |
| `a`   | Add to queue        |
| `x`   | Remove              |
| `/`   | Filter current view |
| `s`   | Spotify search      |
| `D`   | Device switcher     |
| `:`   | Command mode        |

### Other

| Key              | Action              |
|------------------|---------------------|
| `N`              | Now Playing view    |
| `V`              | Toggle vinyl mode (in Now Playing) |
| `Backspace` / `b` | Go back           |
| `?`              | Toggle help overlay |
| `q`              | Quit               |
| `Esc`            | Close / cancel      |

## Commands

Enter command mode by pressing `:`, then type a command.

| Command                 | Description          |
|-------------------------|----------------------|
| `:vol <0-100>`          | Set volume           |
| `:shuffle`              | Toggle shuffle       |
| `:repeat off\|all\|one` | Set repeat mode      |
| `:device`               | Open device switcher |
| `:search <query>`       | Search Spotify       |
| `:recent`               | Recently played      |
| `:q`                    | Quit                 |

## Using Your Own Spotify App (Optional)

waxon works out of the box with no configuration — it ships with a shared client ID used by several open-source Spotify clients. Most users don't need to change anything.

If you'd prefer to use your own Spotify developer app:

1. Go to the [Spotify Developer Dashboard](https://developer.spotify.com/dashboard) and create an app
2. Set the redirect URI to `http://127.0.0.1` (any port — waxon picks one automatically)
3. Copy the **Client ID** and run setup with it:

   ```
   SPOTIFY_CLIENT_ID=your_client_id waxon auth
   ```

The client ID is saved to `~/.config/waxon/config.json` automatically, so you only need to set the environment variable once during setup.

## Environment Variables

| Variable            | Description                        |
|---------------------|------------------------------------|
| `SPOTIFY_CLIENT_ID` | Override the saved Spotify Client ID |
| `WAXON_LOG`         | Path to debug log file (e.g. `/tmp/waxon.log`) |

## CLI Usage

```
waxon          Launch the TUI
waxon auth    Connect or re-connect your Spotify account
waxon version  Print version
waxon help     Show this help
```

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE).
