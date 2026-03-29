# spotui — A Mood-Reactive Terminal Music Companion

## Overview

spotui is a TUI app built with Go + Bubble Tea + Lipgloss that transforms your terminal's entire atmosphere based on whatever music is playing on Spotify. The terminal shifts colors, patterns, animation speed, and mood words in real time as songs change. It's not a Spotify client — it's an ambient, atmospheric companion that makes your terminal feel alive.

**Core principle:** The terminal *becomes* the music. Same app, completely different look depending on what's playing.

## Goals

- **Showcase project** — visual wow factor is the top priority. "Wait, that's running in a terminal?"
- **Zero-friction start** — works immediately via macOS `osascript` with no auth or API keys
- **Optional depth** — connect Spotify API for richer, more accurate mood detection
- **Personality** — this should feel crafted and intentional, not like a dashboard

## Architecture

### Elm Architecture (Bubble Tea)

- **Model** — current track info, computed mood, active color palette, animation tick state, Spotify connection status
- **Update** — handles: poll timer ticks (check what's playing), mood transitions (smooth interpolation between palettes), keyboard input (controls), window resize
- **View** — renders the full atmospheric scene: background pattern, track info, vibe bars, mood word, progress bar, controls

### Mood Engine

Standalone package that takes track metadata (and optionally audio features) and outputs a `Mood`:

```go
type Mood struct {
    Name        string   // "warm", "electric", "drift", etc.
    Primary     Color    // dominant palette color
    Secondary   Color    // accent
    Background  Color    // terminal bg tint
    PatternChar string   // "~ ·", "╱╲", "≋ ~", etc.
    Energy      float64  // 0.0-1.0, drives animation speed
}
```

### Track Source Interface

```go
type TrackSource interface {
    CurrentTrack() (*Track, error)
    Play() error
    Pause() error
    Next() error
    Previous() error
}
```

Two implementations:
- `LocalSource` — uses `osascript` to poll Spotify desktop app (~1-2s interval)
- `APISource` — uses Spotify Web API for track + audio features (when authed)

The app doesn't care which is active — it asks for the current track and renders the mood.

## Mood System

### Mood Categories

| Mood | Triggers | Palette | Pattern | Energy |
|------|----------|---------|---------|--------|
| **Warm** | Acoustic, folk, indie, low energy | Amber/gold | `~ · ~ ·` | 0.2-0.4 |
| **Electric** | Electronic, dance, pop, high energy | Purple/magenta | `╱╲╱╲` | 0.7-1.0 |
| **Drift** | Ambient, chill, lo-fi, very low energy | Teal/cyan | `≋ ~ ≋ ~` | 0.1-0.3 |
| **Dark** | Metal, dark rock, intense, minor key | Deep red/crimson | `▪ ▫ ▪ ▫` | 0.5-0.8 |
| **Golden** | Jazz, soul, R&B, groovy | Rich orange/bronze | `♪ · ♫ ·` | 0.3-0.6 |
| **Bright** | Pop-punk, upbeat indie, major key | Coral/pink/yellow | `✦ · ✦ ·` | 0.5-0.7 |

### Heuristic Detection (no API)

- Curated artist-to-mood mapping: well-known artists mapped to their typical mood (e.g., Bon Iver → Warm, M83 → Electric)
- Keyword matching on track/artist/album names for mood hints (e.g., "chill", "remix", "acoustic")
- Default to **Warm** when uncertain (safest, always looks good)

### API-Enhanced Detection (with Spotify auth)

- `energy` + `valence` → primary mood axis (high energy + high valence = Bright, high energy + low valence = Dark)
- `acousticness` → Warm bias
- `tempo` → animation speed scaling
- `danceability` → pattern density

### Mood Transitions

- When a new song triggers a different mood, don't snap — **spring-animate between palettes** using `harmonica` for natural, physics-based transitions
- Interpolate colors, gradually swap pattern characters, ease the animation speed
- This smooth shift is the signature "wow" moment

## UI Layout

Full-screen Bubble Tea alt-screen mode:

```
┌──────────────────────────────────────────┐
│                                          │
│   (background pattern fills the space,   │
│    faded, slowly animating)              │
│                                          │
│   ♫ NOW PLAYING                          │
│   Track Name                             │
│   Artist                                 │
│                                          │
│   ▐█▌▐██▌▐█▌▐███▌▐█▌▐██▌▐█▌  (vibe bars)│
│                                          │
│   ━━━━━━━━━━━━━━━━━━━━━━ 2:14 / 5:36    │
│   ⏮  ⏸  ⏭     ♡   ↻                    │
│                                          │
│                              mood word   │
└──────────────────────────────────────────┘
```

- Background pattern fills the entire terminal, rendered faintly behind everything
- Track info is vertically centered, left-aligned with generous padding
- Vibe bars pulse at a rate driven by the mood's energy value
- Mood word sits bottom-right, large and faded — atmospheric, not informational
- Responsive to terminal size

### Keyboard Controls

- `space` — play/pause
- `n` / `p` — next/previous track
- `q` / `ctrl+c` — quit
- `a` — run auth flow (connect Spotify API)
- `?` — help overlay

### Animations

- **Vibe bars:** each bar's height oscillates on a tick, speed tied to `Energy`
- **Background pattern:** slowly scrolls or shifts (1-2 chars per second)
- **Mood transitions:** smooth color lerp over ~2 seconds when track changes mood

### Idle State

When no music is playing, the app enters a "rest" mode — very slow, dim, neutral palette ambient animation. Almost like the app is sleeping. When music starts, it wakes up smoothly.

## Spotify Integration

### Local Source (zero setup, default)

Uses `osascript` to query Spotify desktop app:
- Poll every 1-2 seconds for: track name, artist, album, playback state, position, duration
- Playback control (play/pause/next/prev) also via `osascript`
- Falls back gracefully if Spotify isn't running — shows idle state

### API Source (opt-in)

`spotui auth` triggers OAuth PKCE flow:
- Opens browser for Spotify login
- Runs a tiny local callback server to capture the token
- Stores refresh token in `~/.config/spotui/auth.json`
- On next launch, detects stored credentials and upgrades to API source automatically
- Falls back to local source if token expires or API is unreachable

## Project Structure

```
spotui/
├── main.go                  # entry point, CLI args
├── app/
│   ├── model.go             # Bubble Tea model, Init/Update/View
│   ├── keys.go              # keybindings
│   └── view.go              # rendering logic (layout, patterns, bars)
├── mood/
│   ├── mood.go              # Mood struct, palette definitions
│   ├── detect.go            # heuristic detection from metadata
│   ├── detect_api.go        # enhanced detection from audio features
│   └── transition.go        # color lerp, smooth mood transitions
├── source/
│   ├── source.go            # TrackSource interface, Track struct
│   ├── local.go             # osascript-based local source
│   └── api.go               # Spotify Web API source
├── auth/
│   ├── oauth.go             # PKCE flow, token storage
│   └── config.go            # ~/.config/spotui/ paths
└── visual/
    ├── pattern.go           # background pattern generation/scrolling
    ├── bars.go              # vibe bar animation
    └── palette.go           # color utilities, lerp helpers
```

## CLI Interface

- `spotui` — launch the TUI (main experience)
- `spotui auth` — connect Spotify account
- `spotui auth status` — check connection status
- `spotui auth logout` — remove stored credentials

## Tech Stack

- **Go** — language
- `charmbracelet/bubbletea` — TUI framework (Elm architecture)
- `charmbracelet/lipgloss` — styling and layout
- `charmbracelet/bubbles` — reusable components (progress bar, help overlay)
- `charmbracelet/harmonica` — physics-based spring animations for smooth mood transitions
- `charmbracelet/colorprofile` — terminal color capability detection for graceful degradation
- `charmbracelet/log` — structured logging for development/debugging
- `zmb3/spotify/v2` + `golang.org/x/oauth2` — Spotify Web API client (for API source)

## Approach

**Layered: Local First, API Optional.** The app works beautifully out of the box with zero setup via `osascript`. Optionally connect Spotify API via `spotui auth` for richer mood detection using audio features. If connected, moods get more precise. If not, it still works and looks great.
