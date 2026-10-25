# Changelog

All notable changes to this project are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Each release's section is also used verbatim as the GitHub Release notes.

## [Unreleased]

## [1.12.0] - 2026-10-25

### Added

- `"lyrics": false` in config turns lyrics off, so waxon contacts only
  Spotify (lrclib.net is the one other service it uses).
- On macOS, waxon uses the HTTPS proxy set in System Settings when
  `HTTPS_PROXY` isn't set, so it reaches Spotify through a corporate proxy
  as other apps do. `waxon doctor` shows the proxy in use, and says what to
  do when macOS picks one with a PAC script, which waxon can't follow.

### Fixed

- When a proxy or firewall blocks lrclib.net (refusing it, or serving its
  own page in place of lyrics), waxon leaves it alone for ten minutes
  instead of asking on every track; `l` tries again sooner. `waxon doctor`
  now spots the same block instead of calling lrclib reachable.
- The Now Playing hint says `v` for vinyl, the key since 1.11.0.

## [1.11.0] - 2026-10-24

### Keys changed

The keys were reworked around a few rules: lowercase acts on the selection
and Shift on the playing track, `g` only goes to places, Enter opens or
plays what's under the cursor and moves you into it, and each letter means
the same thing everywhere.

| Was | Now |
|---|---|
| `ga` / `gA` / `gp` / `gr`: saved albums / followed artists / podcasts / recently played | **Albums · Artists · Podcasts** under Liked Songs in the Library (or `:albums`, `:artists`, `:podcasts`, `:recent`); `ga` / `gA` go to the selected track's album / artist |
| `1` / `2`: Library / Queue | `gl` / `gq`; digits are counts everywhere |
| `f` in the sidebar liked the playing track | `F` likes the playing track from anywhere; `O` opens its menu |
| `V` (vinyl, in Now Playing) | `v` (`V` still works) |
| `:vol -5` was an error | lowers the volume by 5, as in the CLI |

`:albums`, `:artists`, `:podcasts` and `:recent` still work. In config.json,
`section1` and `section2` are gone; waxon says what replaced them instead of
ignoring your keys.

### Added

- Media keys: the keyboard's play/pause, next and previous keys control
  waxon, and the system's Now Playing (Control Center on macOS, MPRIS on
  Linux desktops) shows the track, its cover and progress, with working
  buttons and scrubbing. Set `"media_keys": false` to turn it off.
- Changing the volume shows a level meter: the part Spotify hasn't
  applied yet is drawn dimly and fills in, with a brief glow, once the new
  level takes effect. The status bar's volume dims and glows the same way.
- `M` switches between playing in waxon and controlling the Spotify app
  on this computer, carrying on from the same point. `:device -` goes back
  to the device you were on before, and `:device <name>` switches to a
  device by name.
- When the Spotify desktop app is installed but closed, the device picker
  offers to open it, and `M` does too: waxon starts it in the background
  and moves playback there once it's online.
- The queue (`gq`) shows in the main pane rather than the narrow sidebar:
  Now playing, Next in queue (what you added) and Next from the playlist,
  with a count and total time. Enter skips forward to a track, as the
  Spotify app does, instead of playing it on its own and dropping the rest
  of the queue; the view refreshes as tracks change.
- Artist pages start with **Play** and **Shuffle**, which play the artist
  on Spotify (most apps can't list an artist's top tracks, so a page had
  nothing to play). Album, artist and show rows show their covers, lists
  of releases are headed Release / Album / Year rather than as tracks, and
  each section line carries its count ("ALBUMS · 5").
- Vinyl mode is a proper record now: black grooved vinyl with the cover on
  its centre label, a reflection that stays put as the record turns under
  it, and a tonearm whose needle moves across the record as the track
  plays. It turns smoothly at 33⅓ rpm (it used to step twice a second at
  about 1 rpm), and only while it's on screen and playing.
- `waxon doctor` warns when waxon's logins sit in a git repository (a
  dotfiles repo, say) or a folder a sync service uploads (Dropbox, iCloud
  Drive, OneDrive, Google Drive), where they could leave the machine.
- The actions menu numbers its rows (`o` then `1` goes to the album) and
  takes letters too: `a` queue, `f` like, `+` add to a playlist, `d`
  remove. Go to Album / Artist come first. Playlists get **Shuffle Play**.
- In search, `Tab` moves to the results, where `a`, `f` and `o` act on the
  selected track.
- Now Playing isn't a dead end: playback keys, `s`, `D`, `M`, `:` and `?`
  work in it, and what they open closes back to it.
- `:play`, `:pause`, `:next`, `:prev`, `:seek ±N|M:SS`, `:vol ±N`, `:mute`,
  `:like [on|off]`, `:shuffle [on|off]` and `:h`, read like the CLI's. As
  you type one, the rest of it shows dimmed (then what it takes, such as
  `0-100 or ±N` for `:vol`), and `Tab` or `→` accepts it.
- `m` mutes and unmutes; a count multiplies a seek (`6]` is +30s); `Esc`
  clears a filter or a half-typed count.
- The cursor skips section lines and blank rows instead of stopping on
  them.

### Changed

- Homebrew installs waxon from a formula that builds it from source again,
  instead of a cask that shipped an unsigned binary and cleared macOS's
  quarantine flag on it. If you installed the cask and trusted the tap, the
  next `brew update` installs the formula for you; then remove the cask with
  `brew uninstall --cask --force waxon`. Otherwise run
  `brew uninstall --cask --force waxon && brew install danfry1/tap/waxon`.
- On Windows, waxon keeps its config and logins in `%AppData%\waxon`, the
  standard place, instead of `%USERPROFILE%\.config\waxon`. An existing
  folder is moved there the first time the new version runs; if it can't
  be moved, waxon carries on using it.
- Artist pages load faster: the artist, their top tracks and their
  discography are fetched at the same time instead of one after another.

### Fixed

- Ctrl-C quits, as in any terminal program.
- Opening a playlist from the sidebar moves to its tracks, so the next
  Enter plays one (it took Enter, `l`, Enter).
- `C-d`/`C-u` move half a page in the sidebar too, and rebound movement
  keys move it.
- `a` in the sidebar no longer queues a track from the list you aren't
  looking at.
- Search results move with `C-n`/`C-p` (and `C-j`/`C-k`) as well as the
  arrows; the hint no longer says j/k, which type into the query.
- One misspelt action in `keys` no longer throws away all your custom keys:
  the rest apply, and waxon says what it skipped, and which keys you've
  given to two actions, in the app rather than behind the full-screen UI.
- The help overlay scrolls (j/k, C-d/C-u, g/G) instead of cutting off on a
  short terminal.
- Now Playing no longer shows the terminal's own background in a box
  around the cover and the text; everything sits on the gradient.
- A toast's dismiss timer no longer hides a newer toast early.
- waxon's player shows as a computer (`[PC]`) in device lists, not a
  speaker.
- Playing from Liked Songs on a Spotify app works: the apps refused it
  ("Restriction violated") as waxon named it; only waxon's own player
  accepted that.
- Moving playback from waxon's player to a Spotify app works. Spotify
  refuses that handover, so waxon starts the same track at the same point
  on the app instead. A device that has only just come online gets a few
  seconds to accept before waxon reports a problem.
- A command Spotify sends to a device that's gone (a Spotify app quit
  while playing) is replayed on a device that's there, as when no device
  is active.
- The device, actions, playlist and search popups no longer show the
  terminal's background colour as a ring around their border.
- An artist or album page that fails to load goes back to the view it was
  opened from, with a message, instead of loading forever. A view that is
  loading no longer shows the previous one's track count and genres.

## [1.10.0] - 2026-10-24

### Added

- waxon can play by itself, with no Spotify app open. With
  [librespot](https://github.com/librespot-org/librespot) installed, waxon
  starts its own Spotify Connect device in the background, plays through it
  whenever nothing else is active, and stops it on quit. It streams at
  320 kbps, the highest quality Spotify offers Premium. The guided setup
  below sets it up (or run `waxon player login`); `waxon player off` turns
  it off. If it can't start, waxon says why in a notice with the fix. Its
  login is kept in `~/.config/waxon/librespot/`, readable only by you.
- A guided, full-screen setup. The first `waxon`, `waxon auth` and
  `waxon player login` open it: connect Spotify (with the dashboard steps,
  a live Client ID check and copy/open shortcuts, and paste-the-address over
  SSH), then choose where music plays. waxon lists the devices on your
  account and offers either your Spotify apps (nothing to install) or its
  own player, which it installs with your package manager when you agree
  and signs in.
- The device picker (`D`) offers **play here with waxon** while waxon's own
  player isn't set up, and opens with just that row when Spotify lists no
  devices, so the player can be set up from inside waxon.
- `waxon player` shows whether the player is installed, signed in and on,
  and the next step if not.
- `waxon doctor` checks everything waxon depends on — config, which Spotify
  app, the login (and whether Spotify still accepts it), devices, the
  player, the lyrics service and the terminal — and prints the fix for
  anything that's wrong. It exits non-zero when something is broken.
- Experimental: real album-art images in kitty and Ghostty. Set
  `"art": "kitty"` (or `"auto"`) in config.json, or run with `WAXON_ART=kitty`.
  The status-bar and Now Playing covers use the kitty graphics protocol's
  Unicode placeholders; everything else keeps the half-block art.
- Podcasts: `gp` (or `:podcasts`) lists the shows you follow, and each opens
  to its episodes (newest first, older ones page in). Episodes play
  in the show, show in the status bar and Now Playing, and can be queued,
  saved with `f` or added to playlists. `gc` while an episode plays opens its
  show.
- Saved Albums (`ga` or `:albums`) and Followed Artists (`gA` or `:artists`)
  open in the track list. Enter opens an album or an artist page, and `/`
  filters by name or artist.
  Followed artists need one more Spotify permission: run `waxon auth` once
  and waxon says so if it's missing.
- `waxon auth` works over SSH: approve in a browser on any machine, then
  paste the address it lands on into the terminal. waxon detects SSH
  sessions, and also falls back to pasting when the callback port is busy.
- `waxon logout` forgets the saved Spotify login (and the built-in player's,
  turning the player off), keeping the rest of your config.
- Playback keys: `+`/`=` and `-` change the volume by 5, `S` toggles shuffle
  and `r` cycles repeat (off → all → one). They work in the main view and in
  Now Playing, and can be rebound as `vol_up`, `vol_down`, `shuffle` and
  `repeat`. Volume moves in round steps (55, 60, 65) and responds at once;
  holding a key follows along at a few changes a second.
- Click the progress bar in the status area to seek.
- Visual mode: `v` in the track list selects a range of tracks. `a` queues
  them in order, `f` likes them, `o` adds them to a playlist and `d` removes
  them from your own playlist.
- Count prefixes in the track list: `5j`, `3k`, `12G` and `12gg` work as in
  vim. While the track list has focus, digits start a count; `1` and `2`
  still switch to Library and Queue from the sidebar, as do `gl` and `gq`
  from anywhere.

### Changed

- Now Playing reopens the way you left it: on the lyrics, the vinyl or the
  cover. `:lyrics` and `:vinyl` open it straight on those.
- Playlists open instantly. waxon keeps the first page of each playlist
  you open in its cache directory, along with Spotify's version of it, and
  shows a playlist straight from there when it hasn't changed — no request
  at all. A playlist that changed shows its cached copy at once and refreshes
  in the background.
- waxon starts where you left off: your library, the playlist you had open
  and your place in it appear before Spotify has answered, then update with
  anything that changed. `waxon logout` and `waxon auth` clear the cache.
- The help overlay says its Now Playing keys (`l` lyrics, `V` vinyl) work
  once Now Playing is open, and Now Playing's key hints now list `V`.
- The track list shows placeholder rows while a list first loads, and a
  "loading more" spinner in its header while the next page of a long playlist
  is fetched.
- Now Playing says whether the track has lyrics before you press `l`
  ("l synced lyrics", "l lyrics" or "no lyrics").
- A track played from search now plays inside its album, so playback carries
  on with the rest of the album instead of stopping after one track.
- waxon uses less CPU when idle. The progress tick stops while nothing is
  playing or loading and slows to once a second while the terminal is
  unfocused. Vinyl mode holds still while unfocused, and its frame is reused
  instead of being redrawn on every update.
- The next track's album art is fetched shortly before the current track
  ends, so it shows as soon as the track changes.
- The `/` filter also matches album names, in the track list and the queue.
- A filter no longer carries over when you open another playlist, album or
  artist.
- Typing a filter on a large playlist is cheaper: each track's searchable
  text is lowercased once when it loads instead of on every keystroke.

### Fixed

- Saving a setting no longer wipes a `config.json` that has a syntax error
  (it says to fix the file instead), and keeps settings waxon doesn't know.
- On Windows, signing in opens the whole Spotify link; it used to be cut
  short.
- The first play on a device that was only just activated (often waxon's
  own player) no longer fails with "Restriction violated": waxon retries
  it for a moment while the device gets ready.
- The volume shown after `+`/`-` or `:vol` no longer jumps back to the old
  level on devices that report changes late, such as librespot.
- A stray or stale request to the login callback no longer aborts
  `waxon auth`; it's ignored and the login carries on.
- Filtering a long playlist with `/` now finds tracks that hadn't loaded yet.
  While a filter is active, waxon loads the rest of the playlist in the
  background and matches appear as their pages arrive. Previously the filter
  only searched the pages you had already scrolled past.

## [1.8.1] - 2026-10-06

### Fixed

- Artist pages show the discography again. Spotify now rejects more than 10
  albums per request, so the request failed and the list came back empty.
  waxon pages through it, fetching albums and singles separately so a long
  album list doesn't hide recent singles. Without top tracks (blocked for
  development-mode apps) the page opens on the first album and its header
  summarises the discography.
- Playlists you follow but don't own no longer open as an empty list or a
  "Forbidden" error. Spotify doesn't let development-mode apps list their
  tracks, so waxon says so, shows the track count, and offers what still
  works: Enter plays the playlist, `o` plays it or opens it in Spotify. If
  Spotify refuses playback too, waxon says that rather than reporting a
  missing device.
- Homebrew no longer warns about a deprecated `postflight` stanza when
  loading the waxon cask.

## [1.8.0] - 2026-08-19

### Added

- Guided setup: `waxon auth` (and the first run of `waxon`) asks whether to use
  your own Spotify app or the shared one, and walks through creating a personal
  app step by step — the recommended path, since Spotify rate-limits the shared
  app. `--own`, `--shared` and `--client-id` skip the questions;
  `WAXON_NO_BROWSER=1` prints URLs instead of opening a browser (SSH).

## [1.7.0] - 2026-08-19

### Added

- Nix flake: `nix run github:danfry1/waxon` / `nix profile install github:danfry1/waxon`.
- Scoop bucket for Windows (`scoop bucket add danfry1 …`), and `.deb` / `.rpm` / `.apk`
  packages attached to each release.
- This changelog; GitHub Release notes are generated from it.

### Changed

- Homebrew now ships as a Cask (goreleaser deprecated binary formulae); the install
  command is unchanged and the macOS quarantine flag is cleared on install.
  Existing formula installs (≤ 1.6.0) migrate once with
  `brew uninstall --formula waxon && brew install danfry1/tap/waxon`.
- Built with Go 1.26; CI runs tests on Linux, macOS and Windows, lints with
  golangci-lint v2.12, runs govulncheck and validates the release config. GitHub
  Actions bumped to current majors and Dependabot keeps them (and Go modules)
  updated.

### Fixed

- On Windows the config and token paths were relative to the working directory
  (`$HOME` is unset there); they now resolve under the user's profile directory.

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

[Unreleased]: https://github.com/danfry1/waxon/compare/v1.12.0...HEAD
[1.12.0]: https://github.com/danfry1/waxon/compare/v1.11.0...v1.12.0
[1.11.0]: https://github.com/danfry1/waxon/compare/v1.10.0...v1.11.0
[1.10.0]: https://github.com/danfry1/waxon/compare/v1.8.1...v1.10.0
[1.8.1]: https://github.com/danfry1/waxon/compare/v1.8.0...v1.8.1
[1.8.0]: https://github.com/danfry1/waxon/compare/v1.7.0...v1.8.0
[1.7.0]: https://github.com/danfry1/waxon/compare/v1.6.0...v1.7.0
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
