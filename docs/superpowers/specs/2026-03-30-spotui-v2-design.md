# spotui v2 — Immersive Terminal Spotify Player

**Date:** 2026-03-30
**Status:** Approved

## Overview

Transform spotui from a minimal now-playing display into an immersive, atmospheric terminal Spotify player with mood-reactive visual effects, functional panels (queue, library, search, devices), and Spotify Web API integration for richer data and cross-platform support.

The goal is maximum showcase impact — the kind of terminal app that makes people say "wait, that's a CLI?"

## Design Principles

- **Atmosphere first** — the terminal should feel alive, not like a static dashboard
- **Mood-reactive everything** — colors, particles, animations all driven by the music
- **Charmbracelet ecosystem** — leverage bubbletea, lipgloss, bubbles, harmonica as much as possible
- **Graceful fallback** — osascript source kept for users who skip Spotify API auth
- **Keyboard-first** — vim-style navigation, every action has a shortcut

## 1. Now Playing — Hero Screen

### Ambient Glow

Album art's dominant colors (already extracted) render as a soft radial gradient surrounding the artwork. Implementation uses concentric rings of Unicode block characters (░▒▓) with decreasing color opacity. The glow shifts color smoothly when mood transitions via the existing spring system.

This is the single biggest visual upgrade — it makes the album art feel like it's projecting light into the terminal void.

### Floating Particle System

Mood-colored Unicode dots (`·`, `•`, `∘`, `⋅`) drift across empty terminal space at varying speeds and opacities. Particle behavior is driven by mood energy:

- **Low energy (drift/ambient):** sparse, slow-moving, mostly `·` and `⋅`
- **Medium energy (warm/golden):** moderate density, gentle drift
- **High energy (electric/bright):** dense, faster movement, larger dots

Particles are rendered into the background layer and don't interfere with the main content layout. They spawn at random positions and drift with slight vertical bias. Color matches the current mood's secondary palette.

### Enhanced Vibe Bars

The existing spring-physics bars are upgraded with:

- **Vertical color gradient** — secondary color at base, primary at peak
- **Glow effect** — tall bars tint the background cell behind them, creating a bloom
- **Actual BPM** — beat phase driven by Spotify's tempo data from audio features instead of estimating from energy
- **Reflection** — subtle mirrored bars below at low opacity (already partially implemented)

### Background Breathing

The terminal background color subtly oscillates in lightness on the beat — a slow sinusoidal pulse tied to the beat phase. Amplitude is proportional to energy (chill songs barely breathe, high-energy songs pulse noticeably). The effect is additive on top of mood transitions.

### Typography & Layout

- Track title: bold, slightly larger feel via spacing
- Artist: dimmer, secondary color
- Progress bar: thin line with a glowing scrubber dot (●) that emits the primary mood color
- "NOW PLAYING" label: very faint, wide letter-spacing
- Mood label: ultra-faint at top, fades in/out during transitions
- Vertical centering with responsive scaling to terminal dimensions

## 2. Panel System

Panels overlay the now-playing screen, triggered by keyboard shortcuts. The now-playing continues animating (dimmed) behind the panel.

### Queue Panel (Q)

- Slides in from the right side, occupying ~50% width
- Shows upcoming tracks from Spotify queue
- Each entry: track name, artist, duration
- Current highlight with accent border
- j/k or ↑/↓ to navigate, Enter to jump to track
- Press Q again or Esc to close

### Library Panel (L)

- Slides in from the left side, occupying ~50% width
- Two-level navigation: playlist list → track list
- Shows user's playlists with track counts
- Enter to drill into playlist, Backspace to go back
- j/k or ↑/↓ to navigate, Enter to play
- Press L again or Esc to close

### Search (/)

- Search bar appears at top of screen
- Real-time search against Spotify API (debounced)
- Results grouped: Tracks, Artists, Albums
- Tab to switch result groups
- Enter to play/open, Esc to cancel

### Device Picker (D)

- Small centered overlay listing available Spotify Connect devices
- Shows device name, type, and active status
- Enter to transfer playback, Esc to close

### Panel Design Language

- Panel background is a solid dark color (mood-tinted). The now-playing content outside the panel is rendered at reduced brightness (darker colors, dimmed text) to create a layered feel without actual transparency.
- Same mood-reactive color scheme as now-playing
- Subtle border using lipgloss border styles
- Hint bar at bottom showing available keys
- Smooth slide-in animation using harmonica springs

## 3. Spotify Web API Integration

### Authentication

- OAuth 2.0 PKCE flow (no backend server required)
- Requires a Spotify Developer App client ID (user creates at developer.spotify.com or we bundle one for the project)
- On first run: opens browser to Spotify auth page
- Local HTTP server on random port receives callback
- Tokens stored in `~/.config/spotui/token.json`
- Auto-refresh on expiry using refresh token
- Scopes: `user-read-playback-state`, `user-modify-playback-state`, `user-read-currently-playing`, `playlist-read-private`, `user-library-read`

### API Client

Use `zmb3/spotify/v2` Go library which wraps the Spotify Web API. It provides:

- Playback state (currently playing, queue, devices)
- Playback control (play, pause, next, prev, seek, volume, shuffle, repeat)
- Library access (playlists, liked songs, albums)
- Search (tracks, artists, albums)
- Audio features (energy, valence, danceability, tempo, acousticness)
- Spotify Connect device management

### TrackSource Implementation

New `spotify.PlayerSource` implements the existing `source.TrackSource` interface, making it a drop-in replacement for `source.LocalSource`. The interface may need minor extension for new capabilities (queue, library, search, volume, devices).

### Audio-Features Mood Detection

Replace keyword/artist heuristic detection with Spotify's audio features:

| Mood | Energy | Valence | Danceability | Other |
|------|--------|---------|--------------|-------|
| Warm | 0.2-0.5 | 0.4-0.7 | any | acousticness > 0.5 |
| Electric | > 0.7 | any | > 0.6 | — |
| Drift | < 0.3 | any | < 0.4 | acousticness > 0.3 |
| Dark | > 0.5 | < 0.3 | any | — |
| Golden | 0.3-0.6 | 0.5-0.8 | 0.4-0.7 | — |
| Bright | > 0.5 | > 0.6 | > 0.5 | — |

Fallback to keyword detection when audio features are unavailable (e.g., local files, osascript source).

## 4. Visual Effects System

### Particle Engine (`visual/particles.go`)

- Fixed-size array of particles (configurable, ~30-50 for performance)
- Each particle: x, y (float64), vx, vy (velocity), color (lipgloss.Color), opacity (float64), char (rune)
- Update loop: advance position, wrap around edges, apply mood energy to speed
- Render: stamp particles into a 2D character grid before compositing with main view
- Mood transitions smoothly change particle color and density

### Glow Renderer (`visual/glow.go`)

- Takes: center position, width/height of art, primary/secondary colors
- Renders concentric rings of block characters with exponentially decreasing opacity
- Uses lipgloss background color to tint cells
- Ring characters: `░` (outer) → `▒` (mid) → `▓` (inner, closest to art)
- Glow radius and intensity scale with mood energy

### Charmbracelet Libraries

- **bubbletea** — core Elm architecture (already used)
- **lipgloss** — styling, layout, borders (already used)
- **bubbles** — list component for panels, text input for search, help component
- **harmonica** — spring animations (already used, extend to panel slide-in)
- **log** — structured logging (already used)

Additional libraries to consider:
- **bubbles/list** — for queue and library track lists with built-in filtering
- **bubbles/textinput** — for search bar
- **bubbles/viewport** — for scrollable content in panels

## 5. New Keybindings

| Key | Action |
|-----|--------|
| `space` | Play / Pause |
| `n` | Next track |
| `p` | Previous track |
| `←` `→` | Seek ±5 seconds |
| `+` `-` | Volume up/down |
| `s` | Toggle shuffle |
| `r` | Cycle repeat (off → context → track) |
| `q` | Toggle queue panel |
| `l` | Toggle library panel |
| `/` | Open search |
| `d` | Device picker |
| `?` | Help overlay |
| `j` `k` | Navigate in panels |
| `Enter` | Select/play in panels |
| `Esc` | Close panel/overlay |
| `Backspace` | Go back in library |
| `Ctrl+C` | Quit |

## 6. Architecture

### New Packages

```
auth/
  oauth.go       — PKCE flow, browser launch, callback server
  token.go       — Token storage (~/.config/spotui/), refresh logic

spotify/
  client.go      — Spotify API client wrapper (using zmb3/spotify/v2)
  player.go      — PlayerSource implementing TrackSource interface
  library.go     — Playlist, search, liked songs queries
  features.go    — Audio features fetching and caching
```

### Enhanced Packages

```
app/
  model.go       — Add: panel state enum, active panel model, device list
  view.go        — Add: particle rendering, glow compositing, panel overlay
  keys.go        — Add: new keybindings for panels, volume, shuffle, repeat
  panels.go      — NEW: panel models (queue, library, search, devices)
  effects.go     — NEW: particle tick, glow render, breathing calculation

mood/
  detect.go      — Add: DetectFromFeatures() using audio features data
                   Keep: DetectFromTrack() as fallback

visual/
  particles.go   — NEW: particle system engine
  glow.go        — NEW: radial glow renderer
  bars.go        — Enhance: gradient coloring, glow bloom on tall bars
```

### Kept as Fallback

```
source/
  source.go      — TrackSource interface (unchanged or minimally extended)
  local.go       — osascript implementation (unchanged)
```

### Interface Extensions

The `TrackSource` interface needs extension. Rather than breaking the existing interface, add a new `RichSource` interface that embeds `TrackSource` and adds methods for queue, library, search, volume, and devices. The app checks if the source implements `RichSource` to enable panels; if not (osascript fallback), panels are simply unavailable.

```go
type RichSource interface {
    TrackSource
    Queue() ([]Track, error)
    Playlists() ([]Playlist, error)
    PlaylistTracks(id string) ([]Track, error)
    Search(query string) (*SearchResults, error)
    SetVolume(percent int) error
    Devices() ([]Device, error)
    TransferPlayback(deviceID string) error
    SetShuffle(state bool) error
    SetRepeat(mode string) error
    AudioFeatures(trackID string) (*AudioFeatures, error)
}
```

## 7. Configuration

Token and preferences stored in `~/.config/spotui/`:

```
~/.config/spotui/
  token.json     — OAuth tokens (auto-managed)
  config.toml    — User preferences (optional, sensible defaults)
```

Config options (all optional with defaults):
- `kitty_graphics` — enable Kitty image protocol (default: auto-detect)
- `particle_density` — particle count multiplier (default: 1.0)
- `source` — "spotify" | "local" (default: "spotify" if authenticated, else "local")

## 8. Startup Flow

1. Check for stored token in `~/.config/spotui/token.json`
2. If valid token exists → launch with Spotify API source
3. If no token → show welcome screen with "Press Enter to connect Spotify" or "Press S to skip (local mode)"
4. If connecting → open browser for OAuth, wait for callback, store token, launch
5. If skipping → fall back to osascript local source (macOS only)

## Non-Goals

- Lyrics display (Spotify doesn't expose lyrics via API)
- Audio playback (spotui controls Spotify, it doesn't play audio itself)
- Playlist creation/editing (read-only library access is sufficient)
- Cross-platform osascript alternatives (Linux/Windows users use Spotify API)
