# CLAUDE.md — waxon

## What is this?

waxon is a vim-modal Spotify terminal client built with Go and Bubbletea. It uses PKCE OAuth (no client secret) with a shared ncspot client ID for zero-config setup.

**GitHub:** https://github.com/danfry1/waxon
**Module:** `github.com/danielfry/waxon`

## Build & Test

```bash
make build          # Build binary with version from git tags
make test           # Run tests with race detector
make lint           # Run golangci-lint
make check          # Format + lint + test (full CI locally)
make cover          # Tests with coverage report
```

Or directly:

```bash
go build -o waxon .
go test ./...
go vet ./...
```

## Project Structure

```
main.go             CLI entry point — auth, debug, version, help subcommands
app/                Bubbletea TUI (all UI lives here)
  app.go            Root model, Update loop, View rendering
  sidebar.go        Left pane — library playlists + queue
  tracklist.go      Right pane — track list table
  statusbar.go      Bottom bar — now playing, progress, mode line
  nowplaying.go     Full-screen Now Playing overlay
  albumart.go       Half-block Unicode art renderer
  command.go        : command parser (vol, shuffle, repeat, device, search, recent, q)
  keys.go           Keybindings and g-prefix motion tracker
  search.go         Spotify search overlay
  actions.go        Context actions popup (o key)
  devices.go        Device picker overlay
  help.go           ? help overlay
  toast.go          Floating notification system
  theme.go          Color palette and reusable styles
  navigation.go     Back navigation stack + track cache
  commands.go       Async tea.Cmd functions (fetch playlists, tracks, art, etc.)
  stub_test.go      StubSource + newTestModel helper for all tests
  update_test.go    Main test file (~4500 lines)
auth/               OAuth PKCE flow + token persistence
  oauth.go          Browser-based auth with ephemeral port callback
  token.go          SaveToken/LoadToken + PersistingTokenSource (auto-refresh)
config/             User configuration (merge-safe JSON)
  config.go         Load/Save with atomic writes, preserves existing fields
source/             Interface definitions
  source.go         RichSource interface — all data access goes through this
spotify/            Spotify Web API client
  client.go         OAuth HTTP client setup with PersistingTokenSource
  player.go         Playback control, queue, devices
  library.go        Playlists, tracks, artists, albums, search (raw HTTP)
  features.go       Audio features endpoint
```

## Key Architecture Decisions

- **Single `source.RichSource` interface** — all Spotify API access is behind this interface, making the app testable with `StubSource`
- **Bubbletea Elm architecture** — all state in Model, all mutations in Update, View is pure
- **Sidebar stores `allItems` separately from the live list** — icons survive queue/library toggles
- **Config uses merge-save pattern** — `config.Save()` loads existing config first, only overwrites non-zero fields. Safe to add new settings without clobbering user edits
- **Token auto-refresh** — `PersistingTokenSource` wraps oauth2.TokenSource, saves refreshed tokens to disk transparently

## Auth Flow

Zero-config: uses ncspot's shared client ID (`DefaultClientID` in `auth/oauth.go`). Users can override with `SPOTIFY_CLIENT_ID` env var during `waxon auth`, which persists to `~/.config/waxon/config.json`.

Token stored at `~/.config/waxon/token.json` with `0o600` permissions.

## Conventions

- **No client secret** — PKCE only, no secret in code or config
- **Atomic file writes** — all config/token writes use tmp + rename pattern
- **File permissions** — `0o700` for dirs, `0o600` for files containing tokens/config
- **Error messages** — user-facing errors should suggest the fix (e.g. "Run 'waxon auth' to reconnect")
- **`go vet` and `golangci-lint`** must pass — enforced by lefthook pre-commit and CI
- **Tests run on pre-push** via lefthook

## CI / Release

- **CI** (`.github/workflows/ci.yml`): runs on push to main and PRs — `go test -race`, `go vet`, `golangci-lint`
- **Release** (`.github/workflows/release.yml`): triggered by pushing a `v*` tag — goreleaser builds binaries for macOS/Linux/Windows and pushes Homebrew formula to `danfry1/homebrew-tap`
- **All GitHub Actions pinned to SHAs** for supply-chain security
- **golangci-lint installed from source** in CI (`go install ... @v2.11.4`) because prebuilt binaries are built with Go 1.24 which is lower than our Go 1.25 target

## Release Process

```bash
git tag v1.x.x
git push origin v1.x.x
```

Goreleaser handles everything: builds, GitHub Release, Homebrew tap update.

## Environment Variables

| Variable | Purpose |
|----------|---------|
| `SPOTIFY_CLIENT_ID` | Override the default shared client ID |
| `WAXON_LOG` | Path to debug log file (e.g. `/tmp/waxon.log`) |

## Gotchas

- `go.mod` module path is `github.com/danielfry/waxon` but the GitHub repo is `github.com/danfry1/waxon` — these are different (username vs module path)
- Spotify's queue API pads results with the current track on repeat — `stripTrailingDupes` in `sidebar.go` handles this
- The `list.Model` from bubbles uses value semantics — `Sidebar.Update()` is a value receiver that returns a new Sidebar. Pointer methods like `SetPlaylistIcons` modify in place. Both patterns coexist correctly but be aware of this when adding new sidebar mutations
- `HOMEBREW_TAP_TOKEN` secret on the waxon repo is a fine-grained PAT scoped to `danfry1/homebrew-tap` with Contents read/write only — expires 2027-03-31
