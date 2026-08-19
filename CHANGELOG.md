# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Each release's section is also used verbatim as the GitHub Release notes.

## [Unreleased]

### Added

- Nix flake: `nix run github:danfry1/waxon` / `nix profile install github:danfry1/waxon`.
- Scoop bucket for Windows (`scoop bucket add danfry1 …`), and `.deb` / `.rpm` / `.apk`
  packages attached to each release.
- This changelog; GitHub Release notes are generated from it.

### Changed

- Homebrew now ships as a Cask (goreleaser deprecated binary formulae); the install
  command is unchanged and the macOS quarantine flag is cleared on install.
- Built with Go 1.26; CI runs tests on Linux, macOS and Windows, lints with
  golangci-lint v2.12, runs govulncheck and validates the release config. GitHub
  Actions bumped to current majors and Dependabot keeps them (and Go modules)
  updated.

## [1.6.0] - 2026-08-19

### Added

- **Scriptable CLI.** Every playback action is a subcommand for status bars, hotkeys
  and scripts: `waxon status [--json|--waybar|--format T] [--liked]`,
  `play [query|uri]`, `pause`, `toggle`, `next`, `prev`, `seek`, `vol`, `shuffle`,
  `repeat`, `like`, `queue`, `search`, `devices`, `device`. tmux, waybar and skhd
  snippets are in the README. `waxon demo <cmd>` runs them against demo data. (#15)
- **Themes and configuration.** `~/.config/waxon/config.json` takes `theme`
  (`spotify`, `catppuccin-mocha`, `catppuccin-latte`, `gruvbox`, `tokyonight`,
  `nord`, `dracula`, `rose-pine`), per-colour `colors` overrides and `keys`
  rebinding. `:theme <name>` switches live and saves. `waxon themes` and
  `waxon config`. (#16)
- **Playlist management.** `o` → *Add to Playlist…* (type-to-filter picker),
  *Remove from <playlist>* (only the selected row — duplicates are left alone),
  `:playlist new <name>`. Requires two new Spotify permissions: run `waxon auth`
  once after upgrading; waxon prompts when needed. (#20, #21)
- **No-active-device recovery.** Playing with no Spotify open activates the only
  available device, or asks which one, instead of failing with a raw API error.
  Premium-required is explained. (#13)
- Narrow terminals: one pane at a time below 80 columns (`h`/`l`/`Tab` switch),
  responsive track columns, Now Playing art scales to fit, and a
  "terminal too small" guard below 40×10. (#18)
- `waxon status --waybar` emits the `text/alt/class/tooltip/percentage` object
  waybar's custom modules expect. (#15)
- Troubleshooting section in the README. (#22)

### Changed

- **Personal client IDs are now recommended and fully supported.** The bundled
  shared client ID is rate-limited by Spotify app-wide; waxon explains this and
  points at the two-minute setup of a personal one. Library and playlist calls use
  Spotify's current endpoints (`/me/library`, `/playlists/{id}/items`,
  `/me/playlists`) with legacy fallbacks, so liking and playlist editing work on
  development-mode apps. The one endpoint Spotify still blocks for such apps —
  artist top tracks — degrades to a discography-only artist page. (#22, #23, #24)
- Playback polling adapts to state (2.5 s playing, 8 s paused, 10 s idle, 20 s
  unfocused), post-control refresh bursts are coalesced, and 429s pause polling
  for Spotify's `Retry-After` instead of being reported as connection issues. (#19)
- Album art is cached on disk (`$XDG_CACHE_HOME/waxon/images`, 64 MB) and in
  memory, fetched at the size actually rendered (64 px for icons, 300 px for
  Now Playing), and sidebar icons stream in as they arrive. (#17)
- The help overlay is generated from the live keymap (no more drift); `q` closes it.
  Rebound keys show in help. (#14, #16)
- Like/unlike works on any track, not just the playing one, and looks up the
  real state first. (#14)
- Colours degrade on 256-colour terminals and are dropped under `NO_COLOR`;
  album art falls back to the 256-colour cube or is skipped. True colour is no
  longer required. (#16)
- Search results play inside their album so playback continues afterwards. (#15)

### Fixed

- Going back from a partially loaded playlist then scrolling could splice the
  wrong playlist's pages into the view. (#14)
- Queue section showed stale library rows until the fetch returned; `/` on the
  queue filtered the library instead; a background queue refresh dropped the
  active filter. (#14)
- Zero search results rendered "Searching…" forever. (#14)
- An empty playback poll zeroed volume/shuffle/repeat; artwork failures reset
  the poll backoff and triggered extra polls. (#14)
- `Retry-After` values above 10 s were replaced with 1 s. (#14)
- Lyrics view could panic at the minimum terminal height. (#18)
- Adding to a playlist didn't update the sidebar count (Spotify's listing lags
  writes; counts update optimistically and reconcile later). (#21)

### Removed

- Dead audio-features code (the endpoint is deprecated for new apps). (#14)

## [1.5.0] - 2026-06-13

### Added

- Time-synced lyrics in Now Playing via lrclib (`l`), with plain-lyrics and
  no-lyrics fallbacks. (#10)

### Changed

- Control actions re-poll playback promptly; transient poll errors no longer
  toast on every tick. (#10)

## [1.4.0] - 2026-06-13

### Added

- Act on the playing track from Now Playing (`f` like, `a` queue, `o` actions);
  `gc` jumps to the playing track's playlist/album even when not loaded. (#7)

### Fixed

- Auth callback redirect URI locked to the registered value (#5). (#7, #8)
- Crash when track data arrived before the first window size. (#7)

## [1.3.0] - 2026-04-08

### Added

- Like/unlike tracks from the TUI (`f`). (#6)

## [1.2.2] - 2026-04-06

### Fixed

- Go module path matches the GitHub repository (`go install` works). (#4)

## [1.2.1] - 2026-04-02

### Fixed

- Auth callback path matches ncspot's registered redirect URI. (#3)

## [1.2.0] - 2026-04-01

### Changed

- Config and token live in `~/.config/waxon` on all platforms.

## [1.1.0] - 2026-04-01

### Added

- Demo mode (`waxon demo`, `-tags demo`) and README recordings. (#1)

### Fixed

- Fixed callback port for custom Spotify app OAuth.

## [1.0.1] - 2026-03-31

### Added

- `-v` / `--version` flags.

### Changed

- GitHub Actions pinned to commit SHAs.

## [1.0.0] - 2026-03-31

Initial release: vim-modal Spotify TUI with PKCE auth, library and queue
sidebar, track list, search, device picker, Now Playing view with album art,
and command mode.

[Unreleased]: https://github.com/danfry1/waxon/compare/v1.6.0...HEAD
[1.6.0]: https://github.com/danfry1/waxon/compare/v1.5.0...v1.6.0
[1.5.0]: https://github.com/danfry1/waxon/compare/v1.4.0...v1.5.0
[1.4.0]: https://github.com/danfry1/waxon/compare/v1.3.0...v1.4.0
[1.3.0]: https://github.com/danfry1/waxon/compare/v1.2.2...v1.3.0
[1.2.2]: https://github.com/danfry1/waxon/compare/v1.2.1...v1.2.2
[1.2.1]: https://github.com/danfry1/waxon/compare/v1.2.0...v1.2.1
[1.2.0]: https://github.com/danfry1/waxon/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/danfry1/waxon/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/danfry1/waxon/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/danfry1/waxon/releases/tag/v1.0.0
