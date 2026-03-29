# spotui Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a mood-reactive terminal music companion that transforms the terminal's atmosphere based on what's playing on Spotify.

**Architecture:** Elm Architecture via Bubble Tea (Model/Update/View). A `source` package polls Spotify via osascript, a `mood` package detects mood from track metadata, a `visual` package renders animated patterns and bars, and an `app` package ties it all together as a full-screen TUI. Harmonica provides spring-based animations for smooth mood transitions.

**Tech Stack:** Go, charmbracelet/bubbletea, charmbracelet/lipgloss, charmbracelet/bubbles, charmbracelet/harmonica

---

## File Structure

```
spotui/                              (root: /Users/danielfry/dev/tui)
├── main.go                          # CLI entry point, subcommand routing
├── go.mod
├── go.sum
├── source/
│   ├── source.go                    # Track struct, TrackSource interface
│   ├── local.go                     # LocalSource: osascript-based Spotify polling
│   └── local_test.go                # Tests for output parsing
├── mood/
│   ├── mood.go                      # Mood struct, 6 predefined palettes + idle
│   ├── mood_test.go                 # Tests for palette definitions
│   ├── detect.go                    # DetectMood: heuristic mood from metadata
│   ├── detect_test.go               # Tests for mood detection
│   ├── transition.go                # Transition: harmonica-based smooth shifts
│   └── transition_test.go           # Tests for transition progress
├── visual/
│   ├── palette.go                   # HexToRGB, LerpColor utilities
│   ├── palette_test.go              # Tests for color math
│   ├── pattern.go                   # RenderPattern: mood-specific backgrounds
│   ├── pattern_test.go              # Tests for pattern output
│   ├── bars.go                      # RenderBars: animated vibe bars
│   └── bars_test.go                 # Tests for bar rendering
└── app/
    ├── model.go                     # Bubble Tea Model, Init, Update
    ├── model_test.go                # Tests for Update message handling
    ├── view.go                      # View: full-screen layout rendering
    └── keys.go                      # KeyMap for keybindings + help
```

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: directories `source/`, `mood/`, `visual/`, `app/`

- [ ] **Step 1: Initialize Go module and install dependencies**

```bash
cd /Users/danielfry/dev/tui
go mod init github.com/danielfry/spotui
go get github.com/charmbracelet/bubbletea@latest
go get github.com/charmbracelet/lipgloss@latest
go get github.com/charmbracelet/bubbles@latest
go get github.com/charmbracelet/harmonica@latest
go get github.com/charmbracelet/log@latest
```

- [ ] **Step 2: Create directory structure**

```bash
mkdir -p source mood visual app
```

- [ ] **Step 3: Write minimal main.go**

```go
// main.go
package main

import "fmt"

func main() {
	fmt.Println("spotui — your music, your mood, your terminal")
}
```

- [ ] **Step 4: Verify it builds and runs**

Run: `go run main.go`
Expected: `spotui — your music, your mood, your terminal`

- [ ] **Step 5: Commit**

```bash
git init
echo -e "# spotui\n\nA mood-reactive terminal music companion." > README.md
echo -e ".superpowers/\n.DS_Store" > .gitignore
git add go.mod go.sum main.go README.md .gitignore docs/
git commit -m "feat: scaffold spotui project with dependencies"
```

---

### Task 2: Track Source Types & Local Source

**Files:**
- Create: `source/source.go`
- Create: `source/local.go`
- Create: `source/local_test.go`

- [ ] **Step 1: Write the Track struct and TrackSource interface**

```go
// source/source.go
package source

import "time"

type Track struct {
	Name     string
	Artist   string
	Album    string
	Duration time.Duration
	Position time.Duration
	Playing  bool
}

type TrackSource interface {
	CurrentTrack() (*Track, error)
	Play() error
	Pause() error
	Next() error
	Previous() error
}
```

- [ ] **Step 2: Write tests for osascript output parsing**

```go
// source/local_test.go
package source

import (
	"testing"
	"time"
)

func TestParseOutput(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   *Track
		wantOk bool
	}{
		{
			name:   "not running",
			raw:    "not_running",
			want:   nil,
			wantOk: true,
		},
		{
			name:   "stopped",
			raw:    "stopped",
			want:   nil,
			wantOk: true,
		},
		{
			name: "playing track",
			raw:  "Holocene\nBon Iver\nBon Iver\n336000\n124.5\nplaying",
			want: &Track{
				Name:     "Holocene",
				Artist:   "Bon Iver",
				Album:    "Bon Iver",
				Duration: 336 * time.Second,
				Position: 124*time.Second + 500*time.Millisecond,
				Playing:  true,
			},
			wantOk: true,
		},
		{
			name: "paused track",
			raw:  "Midnight City\nM83\nHurry Up, We're Dreaming\n243000\n60.0\npaused",
			want: &Track{
				Name:     "Midnight City",
				Artist:   "M83",
				Album:    "Hurry Up, We're Dreaming",
				Duration: 243 * time.Second,
				Position: 60 * time.Second,
				Playing:  false,
			},
			wantOk: true,
		},
		{
			name:   "malformed output",
			raw:    "garbage",
			want:   nil,
			wantOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseOutput(tt.raw)
			if tt.wantOk && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tt.wantOk && err == nil {
				t.Fatal("expected error, got nil")
			}
			if tt.want == nil && got != nil {
				t.Fatalf("expected nil track, got %+v", got)
			}
			if tt.want == nil {
				return
			}
			if got == nil {
				t.Fatal("expected track, got nil")
			}
			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Artist != tt.want.Artist {
				t.Errorf("Artist = %q, want %q", got.Artist, tt.want.Artist)
			}
			if got.Album != tt.want.Album {
				t.Errorf("Album = %q, want %q", got.Album, tt.want.Album)
			}
			if got.Duration != tt.want.Duration {
				t.Errorf("Duration = %v, want %v", got.Duration, tt.want.Duration)
			}
			if got.Position != tt.want.Position {
				t.Errorf("Position = %v, want %v", got.Position, tt.want.Position)
			}
			if got.Playing != tt.want.Playing {
				t.Errorf("Playing = %v, want %v", got.Playing, tt.want.Playing)
			}
		})
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./source/ -v`
Expected: FAIL — `parseOutput` not defined

- [ ] **Step 4: Implement LocalSource with parseOutput**

```go
// source/local.go
package source

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const spotifyScript = `if application "Spotify" is running then
	tell application "Spotify"
		if player state is stopped then
			return "stopped"
		end if
		return (name of current track) & "\n" & (artist of current track) & "\n" & (album of current track) & "\n" & (duration of current track) & "\n" & (player position) & "\n" & (player state as string)
	end tell
else
	return "not_running"
end if`

type LocalSource struct{}

func NewLocalSource() *LocalSource {
	return &LocalSource{}
}

func (s *LocalSource) CurrentTrack() (*Track, error) {
	out, err := runOsascript(spotifyScript)
	if err != nil {
		return nil, nil // Spotify not reachable, treat as idle
	}
	return parseOutput(out)
}

func (s *LocalSource) Play() error {
	_, err := runOsascript(`tell application "Spotify" to play`)
	return err
}

func (s *LocalSource) Pause() error {
	_, err := runOsascript(`tell application "Spotify" to pause`)
	return err
}

func (s *LocalSource) Next() error {
	_, err := runOsascript(`tell application "Spotify" to next track`)
	return err
}

func (s *LocalSource) Previous() error {
	_, err := runOsascript(`tell application "Spotify" to previous track`)
	return err
}

func runOsascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseOutput(raw string) (*Track, error) {
	if raw == "not_running" || raw == "stopped" {
		return nil, nil
	}

	lines := strings.SplitN(raw, "\n", 6)
	if len(lines) != 6 {
		return nil, fmt.Errorf("unexpected output format: got %d lines", len(lines))
	}

	durationMs, err := strconv.ParseFloat(lines[3], 64)
	if err != nil {
		return nil, fmt.Errorf("parse duration: %w", err)
	}

	positionSec, err := strconv.ParseFloat(lines[4], 64)
	if err != nil {
		return nil, fmt.Errorf("parse position: %w", err)
	}

	return &Track{
		Name:     lines[0],
		Artist:   lines[1],
		Album:    lines[2],
		Duration: time.Duration(durationMs) * time.Millisecond,
		Position: time.Duration(positionSec * float64(time.Second)),
		Playing:  lines[5] == "playing",
	}, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./source/ -v`
Expected: PASS — all 5 test cases

- [ ] **Step 6: Commit**

```bash
git add source/
git commit -m "feat: add Track type, TrackSource interface, and LocalSource with osascript"
```

---

### Task 3: Mood Types & Palettes

**Files:**
- Create: `mood/mood.go`
- Create: `mood/mood_test.go`

- [ ] **Step 1: Write tests for mood palette definitions**

```go
// mood/mood_test.go
package mood

import "testing"

func TestAllMoodsDefined(t *testing.T) {
	moods := []Mood{Warm, Electric, Drift, Dark, Golden, Bright, Idle}

	for _, m := range moods {
		t.Run(m.Name, func(t *testing.T) {
			if m.Name == "" {
				t.Error("Name is empty")
			}
			if m.Primary == "" {
				t.Error("Primary color is empty")
			}
			if m.Secondary == "" {
				t.Error("Secondary color is empty")
			}
			if m.Background == "" {
				t.Error("Background color is empty")
			}
			if m.PatternChar == "" {
				t.Error("PatternChar is empty")
			}
			if m.Energy < 0 || m.Energy > 1 {
				t.Errorf("Energy = %f, want 0.0-1.0", m.Energy)
			}
		})
	}
}

func TestMoodByName(t *testing.T) {
	got, ok := ByName("electric")
	if !ok {
		t.Fatal("expected to find 'electric'")
	}
	if got.Name != "electric" {
		t.Errorf("Name = %q, want %q", got.Name, "electric")
	}

	_, ok = ByName("nonexistent")
	if ok {
		t.Error("expected 'nonexistent' to not be found")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./mood/ -v`
Expected: FAIL — types not defined

- [ ] **Step 3: Implement Mood struct and all palette definitions**

```go
// mood/mood.go
package mood

type Mood struct {
	Name        string
	Primary     string  // hex color e.g. "#d4a854"
	Secondary   string  // hex accent color
	Background  string  // hex background tint
	PatternChar string  // repeating pattern characters
	Energy      float64 // 0.0-1.0, drives animation speed
}

var (
	Warm = Mood{
		Name:        "warm",
		Primary:     "#d4a854",
		Secondary:   "#8b7a54",
		Background:  "#1a1510",
		PatternChar: "~ · ",
		Energy:      0.3,
	}
	Electric = Mood{
		Name:        "electric",
		Primary:     "#e040fb",
		Secondary:   "#9575cd",
		Background:  "#0d0a1a",
		PatternChar: "╱╲",
		Energy:      0.85,
	}
	Drift = Mood{
		Name:        "drift",
		Primary:     "#26c6da",
		Secondary:   "#4db6ac",
		Background:  "#0a1520",
		PatternChar: "≋ ~ ",
		Energy:      0.15,
	}
	Dark = Mood{
		Name:        "dark",
		Primary:     "#ef5350",
		Secondary:   "#b71c1c",
		Background:  "#1a0a0a",
		PatternChar: "▪ ▫ ",
		Energy:      0.65,
	}
	Golden = Mood{
		Name:        "golden",
		Primary:     "#ffab40",
		Secondary:   "#ff6e40",
		Background:  "#1a1208",
		PatternChar: "♪ · ♫ · ",
		Energy:      0.45,
	}
	Bright = Mood{
		Name:        "bright",
		Primary:     "#ff6b9d",
		Secondary:   "#ffd93d",
		Background:  "#1a1018",
		PatternChar: "✦ · ",
		Energy:      0.6,
	}
	Idle = Mood{
		Name:        "idle",
		Primary:     "#555555",
		Secondary:   "#333333",
		Background:  "#0a0a0a",
		PatternChar: "· ",
		Energy:      0.05,
	}
)

var allMoods = []Mood{Warm, Electric, Drift, Dark, Golden, Bright}

func ByName(name string) (Mood, bool) {
	for _, m := range allMoods {
		if m.Name == name {
			return m, true
		}
	}
	if name == "idle" {
		return Idle, true
	}
	return Mood{}, false
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./mood/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mood/mood.go mood/mood_test.go
git commit -m "feat: define Mood struct with 6 palettes + idle"
```

---

### Task 4: Heuristic Mood Detection

**Files:**
- Create: `mood/detect.go`
- Create: `mood/detect_test.go`

- [ ] **Step 1: Write tests for mood detection**

```go
// mood/detect_test.go
package mood

import "testing"

func TestDetectMood(t *testing.T) {
	tests := []struct {
		name     string
		artist   string
		track    string
		album    string
		wantMood string
	}{
		{"known warm artist", "Bon Iver", "Holocene", "Bon Iver", "warm"},
		{"known electric artist", "M83", "Midnight City", "Hurry Up", "electric"},
		{"known drift artist", "Brian Eno", "Music for Airports", "Ambient 1", "drift"},
		{"known dark artist", "Nine Inch Nails", "Closer", "The Downward Spiral", "dark"},
		{"known golden artist", "Miles Davis", "So What", "Kind of Blue", "golden"},
		{"keyword: acoustic", "Unknown", "acoustic session", "Album", "warm"},
		{"keyword: remix", "Unknown", "song (remix)", "Album", "electric"},
		{"keyword: chill", "Unknown", "chill vibes", "Album", "drift"},
		{"keyword: live jazz", "Unknown", "live at the club", "Jazz Night", "golden"},
		{"unknown defaults to warm", "Nobody Special", "Random Song", "Random Album", "warm"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectMood(tt.artist, tt.track, tt.album)
			if got.Name != tt.wantMood {
				t.Errorf("DetectMood(%q, %q, %q) = %q, want %q",
					tt.artist, tt.track, tt.album, got.Name, tt.wantMood)
			}
		})
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./mood/ -run TestDetectMood -v`
Expected: FAIL — `DetectMood` not defined

- [ ] **Step 3: Implement heuristic mood detection**

```go
// mood/detect.go
package mood

import "strings"

var artistMoods = map[string]Mood{
	// Warm — acoustic, folk, indie
	"bon iver":       Warm,
	"iron & wine":    Warm,
	"fleet foxes":    Warm,
	"sufjan stevens": Warm,
	"jose gonzalez":  Warm,
	"nick drake":     Warm,
	"elliott smith":  Warm,
	"phoebe bridgers": Warm,
	"big thief":      Warm,
	"the lumineers":  Warm,

	// Electric — electronic, dance, synth
	"m83":            Electric,
	"daft punk":      Electric,
	"the weeknd":     Electric,
	"deadmau5":       Electric,
	"justice":        Electric,
	"lcd soundsystem": Electric,
	"tame impala":    Electric,
	"charli xcx":     Electric,
	"disclosure":     Electric,
	"flume":          Electric,

	// Drift — ambient, chill, lo-fi
	"brian eno":      Drift,
	"tycho":          Drift,
	"boards of canada": Drift,
	"aphex twin":     Drift,
	"nils frahm":     Drift,
	"olafur arnalds": Drift,
	"sigur ros":      Drift,
	"bonobo":         Drift,
	"khruangbin":     Drift,

	// Dark — metal, dark rock, intense
	"nine inch nails":  Dark,
	"tool":             Dark,
	"radiohead":        Dark,
	"massive attack":   Dark,
	"portishead":       Dark,
	"nick cave":        Dark,
	"depeche mode":     Dark,
	"type o negative":  Dark,
	"black sabbath":    Dark,

	// Golden — jazz, soul, R&B
	"miles davis":       Golden,
	"john coltrane":     Golden,
	"nina simone":       Golden,
	"erykah badu":       Golden,
	"d'angelo":          Golden,
	"bill evans":        Golden,
	"amy winehouse":     Golden,
	"anderson .paak":    Golden,
	"sade":              Golden,
	"frank ocean":       Golden,

	// Bright — upbeat pop, pop-punk, happy
	"haim":              Bright,
	"paramore":          Bright,
	"carly rae jepsen":  Bright,
	"chappell roan":     Bright,
	"bleachers":         Bright,
	"the 1975":          Bright,
	"walk the moon":     Bright,
	"passion pit":       Bright,
}

var keywordMoods = []struct {
	keywords []string
	mood     Mood
}{
	{[]string{"acoustic", "unplugged", "folk", "campfire"}, Warm},
	{[]string{"remix", "edm", "techno", "synth", "electronic", "club"}, Electric},
	{[]string{"chill", "ambient", "lo-fi", "lofi", "sleep", "relax", "meditation"}, Drift},
	{[]string{"metal", "heavy", "doom", "dark", "goth", "industrial"}, Dark},
	{[]string{"jazz", "soul", "funk", "groove", "swing", "blues"}, Golden},
	{[]string{"pop", "bright", "happy", "sunshine", "summer", "dance"}, Bright},
}

func DetectMood(artist, track, album string) Mood {
	// Try artist lookup first
	artistLower := strings.ToLower(artist)
	if m, ok := artistMoods[artistLower]; ok {
		return m
	}

	// Try keyword matching on all fields
	combined := strings.ToLower(artist + " " + track + " " + album)
	for _, km := range keywordMoods {
		for _, kw := range km.keywords {
			if strings.Contains(combined, kw) {
				return km.mood
			}
		}
	}

	// Default to warm
	return Warm
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./mood/ -run TestDetectMood -v`
Expected: PASS — all 10 test cases

- [ ] **Step 5: Commit**

```bash
git add mood/detect.go mood/detect_test.go
git commit -m "feat: add heuristic mood detection from track metadata"
```

---

### Task 5: Color Utilities

**Files:**
- Create: `visual/palette.go`
- Create: `visual/palette_test.go`

- [ ] **Step 1: Write tests for color utilities**

```go
// visual/palette_test.go
package visual

import (
	"testing"
)

func TestHexToRGB(t *testing.T) {
	r, g, b, err := HexToRGB("#ff8800")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r != 255 || g != 136 || b != 0 {
		t.Errorf("got (%d, %d, %d), want (255, 136, 0)", r, g, b)
	}
}

func TestRGBToHex(t *testing.T) {
	got := RGBToHex(255, 136, 0)
	if got != "#ff8800" {
		t.Errorf("got %q, want %q", got, "#ff8800")
	}
}

func TestLerpColor(t *testing.T) {
	// Lerp from black to white at 50%
	got := LerpColor("#000000", "#ffffff", 0.5)
	// Should be middle gray: #7f7f7f or #808080 depending on rounding
	r, g, b, _ := HexToRGB(got)
	if r < 126 || r > 129 || g < 126 || g > 129 || b < 126 || b > 129 {
		t.Errorf("LerpColor black→white 0.5 = %q (%d,%d,%d), want ~#808080", got, r, g, b)
	}

	// Lerp at 0% should return from color
	got0 := LerpColor("#ff0000", "#0000ff", 0.0)
	if got0 != "#ff0000" {
		t.Errorf("LerpColor at 0.0 = %q, want #ff0000", got0)
	}

	// Lerp at 100% should return to color
	got1 := LerpColor("#ff0000", "#0000ff", 1.0)
	if got1 != "#0000ff" {
		t.Errorf("LerpColor at 1.0 = %q, want #0000ff", got1)
	}
}

func TestLerpFloat(t *testing.T) {
	if got := LerpFloat(0.0, 1.0, 0.5); got != 0.5 {
		t.Errorf("LerpFloat(0, 1, 0.5) = %f, want 0.5", got)
	}
	if got := LerpFloat(10.0, 20.0, 0.25); got != 12.5 {
		t.Errorf("LerpFloat(10, 20, 0.25) = %f, want 12.5", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./visual/ -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement color utilities**

```go
// visual/palette.go
package visual

import (
	"fmt"
	"math"
	"strings"
)

func HexToRGB(hex string) (uint8, uint8, uint8, error) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color: %q", hex)
	}
	var r, g, b uint8
	_, err := fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b, err
}

func RGBToHex(r, g, b uint8) string {
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func LerpColor(from, to string, t float64) string {
	t = math.Max(0, math.Min(1, t))
	r1, g1, b1, err1 := HexToRGB(from)
	r2, g2, b2, err2 := HexToRGB(to)
	if err1 != nil || err2 != nil {
		return from
	}
	r := uint8(math.Round(float64(r1) + t*(float64(r2)-float64(r1))))
	g := uint8(math.Round(float64(g1) + t*(float64(g2)-float64(g1))))
	b := uint8(math.Round(float64(b1) + t*(float64(b2)-float64(b1))))
	return RGBToHex(r, g, b)
}

func LerpFloat(from, to, t float64) float64 {
	return from + t*(to-from)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./visual/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add visual/palette.go visual/palette_test.go
git commit -m "feat: add color lerp and hex/RGB conversion utilities"
```

---

### Task 6: Mood Transitions with Harmonica

**Files:**
- Create: `mood/transition.go`
- Create: `mood/transition_test.go`

- [ ] **Step 1: Write tests for mood transitions**

```go
// mood/transition_test.go
package mood

import "testing"

func TestTransitionStart(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	if !tr.Active {
		t.Error("expected transition to be active")
	}
	if tr.Progress() != 0.0 {
		t.Errorf("initial progress = %f, want 0.0", tr.Progress())
	}
}

func TestTransitionTick(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	// Tick many times to approach completion
	for range 200 {
		tr.Tick()
	}
	if tr.Progress() < 0.95 {
		t.Errorf("after 200 ticks, progress = %f, want >= 0.95", tr.Progress())
	}
}

func TestTransitionDone(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	// Tick until done
	for range 500 {
		tr.Tick()
	}
	if !tr.Done() {
		t.Error("expected transition to be done after 500 ticks")
	}
}

func TestTransitionCurrentMood(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	// At start, current should be close to Warm
	cur := tr.Current()
	if cur.Name != "warm → electric" {
		t.Errorf("Name = %q, want %q", cur.Name, "warm → electric")
	}
	if cur.Energy < Warm.Energy-0.01 || cur.Energy > Warm.Energy+0.01 {
		t.Errorf("initial energy = %f, want ~%f", cur.Energy, Warm.Energy)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./mood/ -run TestTransition -v`
Expected: FAIL — `NewTransition` not defined

- [ ] **Step 3: Implement Transition**

```go
// mood/transition.go
package mood

import (
	"fmt"

	"github.com/charmbracelet/harmonica"
	"github.com/danielfry/spotui/visual"
)

const doneThreshold = 0.99

type Transition struct {
	From     Mood
	To       Mood
	Active   bool
	pos      float64
	vel      float64
	spring   harmonica.Spring
}

func NewTransition(from, to Mood) *Transition {
	return &Transition{
		From:   from,
		To:     to,
		Active: true,
		pos:    0.0,
		vel:    0.0,
		// FPS(30) = delta time for 30fps, angularFreq=5.0, damping=1.0 (critically damped)
		spring: harmonica.NewSpring(harmonica.FPS(30), 5.0, 1.0),
	}
}

func (t *Transition) Tick() {
	if !t.Active {
		return
	}
	t.pos, t.vel = t.spring.Update(t.pos, t.vel, 1.0)
	if t.pos >= doneThreshold {
		t.pos = 1.0
		t.Active = false
	}
}

func (t *Transition) Progress() float64 {
	return t.pos
}

func (t *Transition) Done() bool {
	return !t.Active
}

func (t *Transition) Current() Mood {
	p := t.pos
	return Mood{
		Name:        fmt.Sprintf("%s → %s", t.From.Name, t.To.Name),
		Primary:     visual.LerpColor(t.From.Primary, t.To.Primary, p),
		Secondary:   visual.LerpColor(t.From.Secondary, t.To.Secondary, p),
		Background:  visual.LerpColor(t.From.Background, t.To.Background, p),
		PatternChar: t.patternAt(p),
		Energy:      visual.LerpFloat(t.From.Energy, t.To.Energy, p),
	}
}

func (t *Transition) patternAt(p float64) string {
	if p < 0.5 {
		return t.From.PatternChar
	}
	return t.To.PatternChar
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./mood/ -run TestTransition -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add mood/transition.go mood/transition_test.go
git commit -m "feat: add harmonica-based mood transitions with spring animation"
```

---

### Task 7: Visual Components — Patterns & Bars

**Files:**
- Create: `visual/pattern.go`
- Create: `visual/pattern_test.go`
- Create: `visual/bars.go`
- Create: `visual/bars_test.go`

- [ ] **Step 1: Write tests for pattern rendering**

```go
// visual/pattern_test.go
package visual

import (
	"strings"
	"testing"
)

func TestRenderPattern(t *testing.T) {
	row := RenderPatternRow("~ · ", 20)
	if len([]rune(row)) != 20 {
		t.Errorf("row rune length = %d, want 20", len([]rune(row)))
	}
	if !strings.Contains(row, "~") {
		t.Error("expected row to contain pattern characters")
	}
}

func TestRenderPatternRows(t *testing.T) {
	rows := RenderPatternRows("╱╲", 30, 5, 0)
	if len(rows) != 5 {
		t.Errorf("got %d rows, want 5", len(rows))
	}
	for i, row := range rows {
		if len([]rune(row)) != 30 {
			t.Errorf("row %d rune length = %d, want 30", i, len([]rune(row)))
		}
	}
}

func TestRenderPatternRowsOffset(t *testing.T) {
	rows0 := RenderPatternRows("~ · ", 20, 1, 0)
	rows1 := RenderPatternRows("~ · ", 20, 1, 1)
	if rows0[0] == rows1[0] {
		t.Error("expected different rows with different offsets")
	}
}
```

- [ ] **Step 2: Write tests for bar rendering**

```go
// visual/bars_test.go
package visual

import (
	"strings"
	"testing"
)

func TestRenderBars(t *testing.T) {
	heights := []float64{0.5, 0.8, 0.3, 1.0, 0.0}
	result := RenderBars(heights, 6, "#ff0000")
	lines := strings.Split(result, "\n")
	if len(lines) != 6 {
		t.Errorf("got %d lines, want 6", len(lines))
	}
}

func TestRenderBarsEmpty(t *testing.T) {
	result := RenderBars(nil, 4, "#ff0000")
	if result == "" {
		t.Error("expected non-empty output even with no bars")
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./visual/ -v`
Expected: FAIL — functions not defined

- [ ] **Step 4: Implement pattern rendering**

```go
// visual/pattern.go
package visual

import "strings"

func RenderPatternRow(pattern string, width int) string {
	if pattern == "" {
		pattern = " "
	}
	runes := []rune(pattern)
	row := make([]rune, width)
	for i := range width {
		row[i] = runes[i%len(runes)]
	}
	return string(row)
}

func RenderPatternRows(pattern string, width, height, offset int) []string {
	if pattern == "" {
		pattern = " "
	}
	runes := []rune(pattern)
	rows := make([]string, height)
	for i := range height {
		shifted := make([]rune, width)
		rowOffset := offset + i
		for j := range width {
			idx := (j + rowOffset) % len(runes)
			shifted[j] = runes[idx]
		}
		rows[i] = string(shifted)
	}
	return rows
}

func RepeatToWidth(s string, width int) string {
	if s == "" {
		return strings.Repeat(" ", width)
	}
	runes := []rune(s)
	result := make([]rune, width)
	for i := range width {
		result[i] = runes[i%len(runes)]
	}
	return string(result)
}
```

- [ ] **Step 5: Implement bar rendering**

```go
// visual/bars.go
package visual

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var barChars = []string{"▁", "▂", "▃", "▄", "▅", "▆", "▇", "█"}

func RenderBars(heights []float64, maxHeight int, color string) string {
	if len(heights) == 0 {
		return strings.Repeat("\n", maxHeight)
	}

	style := lipgloss.NewStyle().Foreground(lipgloss.Color(color))

	lines := make([]string, maxHeight)
	for row := range maxHeight {
		var sb strings.Builder
		for _, h := range heights {
			barH := h * float64(maxHeight)
			rowFromBottom := maxHeight - 1 - row
			if float64(rowFromBottom) < barH-1 {
				sb.WriteString(style.Render("█"))
			} else if float64(rowFromBottom) < barH {
				frac := barH - math.Floor(barH)
				idx := int(frac * float64(len(barChars)-1))
				idx = max(0, min(idx, len(barChars)-1))
				sb.WriteString(style.Render(barChars[idx]))
			} else {
				sb.WriteString(" ")
			}
			sb.WriteString(" ") // gap between bars
		}
		lines[row] = sb.String()
	}
	return strings.Join(lines, "\n")
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./visual/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add visual/pattern.go visual/pattern_test.go visual/bars.go visual/bars_test.go
git commit -m "feat: add pattern rendering and animated vibe bars"
```

---

### Task 8: Keybindings

**Files:**
- Create: `app/keys.go`

- [ ] **Step 1: Define the KeyMap**

```go
// app/keys.go
package app

import "github.com/charmbracelet/bubbles/key"

type KeyMap struct {
	PlayPause key.Binding
	Next      key.Binding
	Prev      key.Binding
	Help      key.Binding
	Quit      key.Binding
}

func DefaultKeyMap() KeyMap {
	return KeyMap{
		PlayPause: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("space", "play/pause"),
		),
		Next: key.NewBinding(
			key.WithKeys("n"),
			key.WithHelp("n", "next track"),
		),
		Prev: key.NewBinding(
			key.WithKeys("p"),
			key.WithHelp("p", "prev track"),
		),
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.PlayPause, k.Next, k.Prev, k.Help, k.Quit}
}

func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.PlayPause, k.Next, k.Prev},
		{k.Help, k.Quit},
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/danielfry/dev/tui && go build ./app/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add app/keys.go
git commit -m "feat: add keybindings with help support"
```

---

### Task 9: Bubble Tea Model & Update

**Files:**
- Create: `app/model.go`
- Create: `app/model_test.go`

- [ ] **Step 1: Write tests for Update message handling**

```go
// app/model_test.go
package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/source"
)

type fakeSource struct {
	track *source.Track
}

func (f *fakeSource) CurrentTrack() (*source.Track, error) { return f.track, nil }
func (f *fakeSource) Play() error                          { return nil }
func (f *fakeSource) Pause() error                         { return nil }
func (f *fakeSource) Next() error                          { return nil }
func (f *fakeSource) Previous() error                      { return nil }

func TestModelInitialState(t *testing.T) {
	m := NewModel(&fakeSource{})
	if m.mood.Name != "idle" {
		t.Errorf("initial mood = %q, want %q", m.mood.Name, "idle")
	}
	if m.track != nil {
		t.Error("expected nil track initially")
	}
}

func TestModelTrackUpdate(t *testing.T) {
	m := NewModel(&fakeSource{})
	track := &source.Track{
		Name:    "Holocene",
		Artist:  "Bon Iver",
		Album:   "Bon Iver",
		Playing: true,
		Duration: 5 * time.Minute,
	}
	updated, _ := m.Update(trackUpdateMsg{track})
	m = updated.(Model)

	if m.track == nil {
		t.Fatal("expected track to be set")
	}
	if m.track.Name != "Holocene" {
		t.Errorf("track name = %q, want %q", m.track.Name, "Holocene")
	}
}

func TestModelTrackClearedOnNil(t *testing.T) {
	m := NewModel(&fakeSource{})
	// Set a track first
	m.track = &source.Track{Name: "Test", Artist: "Test", Playing: true}
	m.mood = mood.Warm

	updated, _ := m.Update(trackUpdateMsg{nil})
	m = updated.(Model)

	if m.track != nil {
		t.Error("expected track to be nil after nil update")
	}
}

func TestModelQuit(t *testing.T) {
	m := NewModel(&fakeSource{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModelHelpToggle(t *testing.T) {
	m := NewModel(&fakeSource{})
	if m.showHelp {
		t.Error("expected showHelp to be false initially")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !m.showHelp {
		t.Error("expected showHelp to be true after ?")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.showHelp {
		t.Error("expected showHelp to be false after second ?")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /Users/danielfry/dev/tui && go test ./app/ -v`
Expected: FAIL — `NewModel` not defined

- [ ] **Step 3: Implement Model, Init, and Update**

```go
// app/model.go
package app

import (
	"math/rand/v2"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/source"
)

const (
	numBars     = 20
	barMaxH     = 6
	animFPS     = 30
	pollSeconds = 1.5
)

type animTickMsg time.Time
type pollTickMsg time.Time
type trackUpdateMsg struct{ track *source.Track }
type trackErrorMsg struct{ err error }
type controlDoneMsg struct{}

type Model struct {
	source     source.TrackSource
	track      *source.Track
	mood       mood.Mood
	targetMood mood.Mood
	transition *mood.Transition
	bars       [numBars]float64
	barVels    [numBars]float64
	barTargets [numBars]float64
	barSprings [numBars]harmonica.Spring
	pattern    int
	width      int
	height     int
	help       help.Model
	showHelp   bool
	keys       KeyMap
	quitting   bool
	lastPoll   time.Time
}

func NewModel(src source.TrackSource) Model {
	m := Model{
		source:     src,
		mood:       mood.Idle,
		targetMood: mood.Idle,
		keys:       DefaultKeyMap(),
		help:       help.New(),
	}
	for i := range numBars {
		m.barSprings[i] = harmonica.NewSpring(harmonica.FPS(animFPS), 8.0, 0.6)
		m.barTargets[i] = rand.Float64() * 0.3
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg { return animTickMsg(t) }),
		tea.Tick(time.Duration(pollSeconds*float64(time.Second)), func(t time.Time) tea.Msg { return pollTickMsg(t) }),
	)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		case key.Matches(msg, m.keys.Help):
			m.showHelp = !m.showHelp
			return m, nil
		case key.Matches(msg, m.keys.PlayPause):
			if m.track != nil && m.track.Playing {
				return m, controlCmd(m.source.Pause)
			}
			return m, controlCmd(m.source.Play)
		case key.Matches(msg, m.keys.Next):
			return m, controlCmd(m.source.Next)
		case key.Matches(msg, m.keys.Prev):
			return m, controlCmd(m.source.Previous)
		}

	case animTickMsg:
		m.tickAnimation()
		return m, tea.Tick(time.Second/animFPS, func(t time.Time) tea.Msg { return animTickMsg(t) })

	case pollTickMsg:
		return m, tea.Batch(
			fetchTrack(m.source),
			tea.Tick(time.Duration(pollSeconds*float64(time.Second)), func(t time.Time) tea.Msg { return pollTickMsg(t) }),
		)

	case trackUpdateMsg:
		m.handleTrackUpdate(msg.track)
		return m, nil

	case trackErrorMsg:
		// Spotify unreachable — go idle
		m.track = nil
		m.startTransitionTo(mood.Idle)
		return m, nil

	case controlDoneMsg:
		return m, nil
	}

	return m, nil
}

func (m *Model) tickAnimation() {
	// Advance pattern scroll
	m.pattern++

	// Animate bars toward targets
	energy := m.mood.Energy
	for i := range numBars {
		m.bars[i], m.barVels[i] = m.barSprings[i].Update(m.bars[i], m.barVels[i], m.barTargets[i])
		// Periodically randomize targets based on energy
		if rand.Float64() < energy*0.15 {
			m.barTargets[i] = rand.Float64() * (0.3 + energy*0.7)
		}
	}

	// Advance mood transition
	if m.transition != nil {
		m.transition.Tick()
		m.mood = m.transition.Current()
		if m.transition.Done() {
			m.mood = m.targetMood
			m.transition = nil
		}
	}
}

func (m *Model) handleTrackUpdate(track *source.Track) {
	m.track = track
	if track == nil {
		m.startTransitionTo(mood.Idle)
		return
	}
	detected := mood.DetectMood(track.Artist, track.Name, track.Album)
	if detected.Name != m.targetMood.Name {
		m.startTransitionTo(detected)
	}
}

func (m *Model) startTransitionTo(target mood.Mood) {
	if m.mood.Name == target.Name {
		return
	}
	m.targetMood = target
	m.transition = mood.NewTransition(m.mood, target)
}

func fetchTrack(src source.TrackSource) tea.Cmd {
	return func() tea.Msg {
		track, err := src.CurrentTrack()
		if err != nil {
			return trackErrorMsg{err}
		}
		return trackUpdateMsg{track}
	}
}

func controlCmd(fn func() error) tea.Cmd {
	return func() tea.Msg {
		fn()
		return controlDoneMsg{}
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /Users/danielfry/dev/tui && go test ./app/ -v`
Expected: PASS — all 5 tests

- [ ] **Step 5: Commit**

```bash
git add app/model.go app/model_test.go
git commit -m "feat: add Bubble Tea model with update loop, polling, and transitions"
```

---

### Task 10: View Rendering

**Files:**
- Create: `app/view.go`

- [ ] **Step 1: Implement the full View method**

```go
// app/view.go
package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danielfry/spotui/visual"
)

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width == 0 || m.height == 0 {
		return "starting..."
	}

	md := m.mood
	primary := lipgloss.Color(md.Primary)
	secondary := lipgloss.Color(md.Secondary)
	bg := lipgloss.Color(md.Background)
	dimColor := lipgloss.Color(visual.LerpColor(md.Background, md.Primary, 0.15))

	// Styles
	patternStyle := lipgloss.NewStyle().Foreground(dimColor).Background(bg)
	titleStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
	trackStyle := lipgloss.NewStyle().Foreground(primary).Bold(true).Background(bg)
	artistStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
	labelStyle := lipgloss.NewStyle().Foreground(primary).Background(bg)
	controlStyle := lipgloss.NewStyle().Foreground(secondary).Background(bg)
	moodStyle := lipgloss.NewStyle().Foreground(dimColor).Background(bg)
	bgStyle := lipgloss.NewStyle().Background(bg)

	contentWidth := min(m.width-8, 60)
	pad := lipgloss.NewStyle().PaddingLeft(4).Background(bg)

	var content strings.Builder

	// "NOW PLAYING" label
	if m.track != nil {
		content.WriteString(pad.Render(labelStyle.Render("♫ NOW PLAYING")))
		content.WriteString("\n")
		content.WriteString(pad.Render(trackStyle.Render(m.track.Name)))
		content.WriteString("\n")
		content.WriteString(pad.Render(artistStyle.Render(m.track.Artist)))
	} else {
		content.WriteString(pad.Render(titleStyle.Render("♫ spotui")))
		content.WriteString("\n")
		content.WriteString(pad.Render(artistStyle.Render("waiting for music...")))
	}
	content.WriteString("\n\n")

	// Vibe bars
	barHeights := m.bars[:]
	barsStr := visual.RenderBars(barHeights, barMaxH, md.Primary)
	for _, line := range strings.Split(barsStr, "\n") {
		content.WriteString(pad.Render(line))
		content.WriteString("\n")
	}
	content.WriteString("\n")

	// Progress bar
	if m.track != nil {
		progress := m.renderProgress(contentWidth, primary, secondary)
		content.WriteString(pad.Render(progress))
		content.WriteString("\n")

		// Controls
		playPause := "▶"
		if m.track.Playing {
			playPause = "⏸"
		}
		controls := controlStyle.Render(fmt.Sprintf("⏮  %s  ⏭", playPause))
		content.WriteString(pad.Render(controls))
	}
	content.WriteString("\n")

	contentStr := content.String()
	contentLines := strings.Split(contentStr, "\n")
	contentH := len(contentLines)

	// Pattern rows fill remaining space
	totalH := m.height
	topPatternH := max(0, (totalH-contentH)/2-1)
	bottomPatternH := max(0, totalH-contentH-topPatternH-2)

	topPatterns := visual.RenderPatternRows(md.PatternChar, m.width, topPatternH, m.pattern)
	bottomPatterns := visual.RenderPatternRows(md.PatternChar, m.width, bottomPatternH, m.pattern+topPatternH)

	var full strings.Builder
	for _, row := range topPatterns {
		full.WriteString(patternStyle.Render(row))
		full.WriteString("\n")
	}
	full.WriteString("\n")

	for _, line := range contentLines {
		// Pad line to full width with background
		lineWidth := lipgloss.Width(line)
		if lineWidth < m.width {
			line += bgStyle.Render(strings.Repeat(" ", m.width-lineWidth))
		}
		full.WriteString(line)
		full.WriteString("\n")
	}

	for _, row := range bottomPatterns {
		full.WriteString(patternStyle.Render(row))
		full.WriteString("\n")
	}

	// Mood word bottom-right
	if bottomPatternH > 0 {
		moodWord := moodStyle.Render(md.Name)
		moodLine := strings.Repeat(" ", max(0, m.width-lipgloss.Width(moodWord)-4)) + moodWord
		full.WriteString(patternStyle.Render(moodLine))
	}

	// Help overlay
	if m.showHelp {
		helpStr := m.help.View(m.keys)
		helpStyled := lipgloss.NewStyle().
			Foreground(secondary).
			Background(bg).
			Padding(1, 2).
			Render(helpStr)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, helpStyled,
			lipgloss.WithWhitespaceBackground(bg))
	}

	return full.String()
}

func (m Model) renderProgress(width int, primary, secondary lipgloss.Color) string {
	if m.track == nil {
		return ""
	}
	pos := m.track.Position
	dur := m.track.Duration
	if dur == 0 {
		return ""
	}

	barWidth := width - 14 // space for timestamps
	filled := int(float64(barWidth) * (float64(pos) / float64(dur)))
	filled = max(0, min(filled, barWidth))
	empty := barWidth - filled

	bar := lipgloss.NewStyle().Foreground(primary).Render(strings.Repeat("━", filled)) +
		lipgloss.NewStyle().Foreground(secondary).Render(strings.Repeat("━", empty))

	posStr := formatDuration(pos)
	durStr := formatDuration(dur)

	return fmt.Sprintf("%s %s / %s", bar, posStr, durStr)
}

func formatDuration(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/danielfry/dev/tui && go build ./app/`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add app/view.go
git commit -m "feat: add full-screen view with patterns, bars, progress, and mood word"
```

---

### Task 11: CLI Entry Point

**Files:**
- Modify: `main.go`

- [ ] **Step 1: Update main.go to launch the TUI**

```go
// main.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/spotui/app"
	"github.com/danielfry/spotui/source"
)

func main() {
	src := source.NewLocalSource()
	m := app.NewModel(src)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/danielfry/dev/tui && go build -o spotui .`
Expected: Binary `spotui` created with no errors

- [ ] **Step 3: Manual smoke test**

Run: `cd /Users/danielfry/dev/tui && ./spotui`

Expected behavior:
- Full-screen alt-screen mode activates
- If Spotify is playing: shows track info, mood colors, animated bars and patterns
- If Spotify is not playing: shows "waiting for music..." in idle palette
- Press `?` to toggle help overlay
- Press `space` to play/pause
- Press `n`/`p` for next/previous
- Press `q` to quit cleanly

- [ ] **Step 4: Commit**

```bash
git add main.go
git commit -m "feat: add CLI entry point that launches the TUI"
```

---

### Task 12: Polish — Idle State & Help Styling

**Files:**
- Modify: `app/view.go`
- Modify: `app/model.go`

- [ ] **Step 1: Enhance idle state with "sleeping" message**

In `app/view.go`, update the idle track section. Replace the block:

```go
	} else {
		content.WriteString(pad.Render(titleStyle.Render("♫ spotui")))
		content.WriteString("\n")
		content.WriteString(pad.Render(artistStyle.Render("waiting for music...")))
	}
```

with:

```go
	} else {
		content.WriteString(pad.Render(titleStyle.Render("♫ spotui")))
		content.WriteString("\n")
		content.WriteString(pad.Render(artistStyle.Render("waiting for music...")))
		content.WriteString("\n")
		content.WriteString(pad.Render(controlStyle.Render("play something on Spotify to begin")))
	}
```

- [ ] **Step 2: Ensure position estimates between polls when playing**

In `app/model.go`, add position estimation to `tickAnimation`. Add this at the end of the method:

```go
	// Estimate position between polls
	if m.track != nil && m.track.Playing {
		m.track.Position += time.Second / animFPS
		if m.track.Position > m.track.Duration {
			m.track.Position = m.track.Duration
		}
	}
```

- [ ] **Step 3: Run all tests**

Run: `cd /Users/danielfry/dev/tui && go test ./... -v`
Expected: All tests PASS

- [ ] **Step 4: Manual smoke test with Spotify**

Run: `cd /Users/danielfry/dev/tui && go run .`

Verify:
- Progress bar updates smoothly between polls
- Idle state shows helpful message
- Help overlay renders with mood colors
- Transition between songs is smooth (~2s spring animation)
- Different genres trigger different moods (play Bon Iver → warm, then M83 → electric)

- [ ] **Step 5: Commit**

```bash
git add app/
git commit -m "feat: polish idle state, smooth progress, and help styling"
```

---

### Task 13: Final Integration & Cleanup

**Files:**
- Modify: `main.go` (add subcommand stubs)
- Run full test suite

- [ ] **Step 1: Add subcommand routing for future auth**

```go
// main.go
package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/spotui/app"
	"github.com/danielfry/spotui/source"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			fmt.Println("spotui auth — Spotify API integration coming soon")
			fmt.Println("For now, spotui works with the Spotify desktop app automatically.")
			return
		case "version":
			fmt.Println("spotui v0.1.0")
			return
		case "help", "--help", "-h":
			printUsage()
			return
		}
	}

	src := source.NewLocalSource()
	m := app.NewModel(src)

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`spotui — a mood-reactive terminal music companion

Usage:
  spotui            Launch the TUI
  spotui auth       Connect Spotify API (coming soon)
  spotui version    Print version
  spotui help       Show this help`)
}
```

- [ ] **Step 2: Run full test suite**

Run: `cd /Users/danielfry/dev/tui && go test ./... -v`
Expected: ALL PASS

- [ ] **Step 3: Build final binary**

Run: `cd /Users/danielfry/dev/tui && go build -o spotui . && ls -la spotui`
Expected: Binary created successfully

- [ ] **Step 4: Final smoke test**

```bash
./spotui version
# Expected: spotui v0.1.0

./spotui help
# Expected: usage text

./spotui
# Expected: full TUI launches
```

- [ ] **Step 5: Commit**

```bash
git add main.go
git commit -m "feat: add CLI subcommands and finalize v0.1.0"
```

---

## Summary

| Task | What it builds | Key files |
|------|---------------|-----------|
| 1 | Project scaffolding | `go.mod`, `main.go` |
| 2 | Track source + osascript | `source/source.go`, `source/local.go` |
| 3 | Mood palettes | `mood/mood.go` |
| 4 | Heuristic detection | `mood/detect.go` |
| 5 | Color utilities | `visual/palette.go` |
| 6 | Mood transitions | `mood/transition.go` |
| 7 | Patterns + bars | `visual/pattern.go`, `visual/bars.go` |
| 8 | Keybindings | `app/keys.go` |
| 9 | Bubble Tea model | `app/model.go` |
| 10 | View rendering | `app/view.go` |
| 11 | CLI entry point | `main.go` |
| 12 | Polish | `app/view.go`, `app/model.go` |
| 13 | Final integration | `main.go` |
