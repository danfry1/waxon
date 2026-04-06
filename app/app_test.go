package app

import (
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

// ---------------------------------------------------------------------------
// GTracker (keys.go)
// ---------------------------------------------------------------------------

func TestGTrackerFeed(t *testing.T) {
	tests := []struct {
		name string
		keys []string
		want []GAction
	}{
		{
			name: "gg produces GActionTop",
			keys: []string{"g", "g"},
			want: []GAction{GActionNone, GActionTop},
		},
		{
			name: "gl produces GActionLibrary",
			keys: []string{"g", "l"},
			want: []GAction{GActionNone, GActionLibrary},
		},
		{
			name: "gq produces GActionQueue",
			keys: []string{"g", "q"},
			want: []GAction{GActionNone, GActionQueue},
		},
		{
			name: "gc produces GActionCurrent",
			keys: []string{"g", "c"},
			want: []GAction{GActionNone, GActionCurrent},
		},
		{
			name: "gr produces GActionRecent",
			keys: []string{"g", "r"},
			want: []GAction{GActionNone, GActionRecent},
		},
		{
			name: "unknown second key resets",
			keys: []string{"g", "z"},
			want: []GAction{GActionNone, GActionNone},
		},
		{
			name: "non-g first key does nothing",
			keys: []string{"x"},
			want: []GAction{GActionNone},
		},
		{
			name: "g then unknown then gg works",
			keys: []string{"g", "z", "g", "g"},
			want: []GAction{GActionNone, GActionNone, GActionNone, GActionTop},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tracker GTracker
			for i, k := range tt.keys {
				got := tracker.Feed(k)
				if got != tt.want[i] {
					t.Errorf("Feed(%q) at step %d = %d, want %d", k, i, got, tt.want[i])
				}
			}
		})
	}
}

func TestGTrackerPending(t *testing.T) {
	var tracker GTracker
	if tracker.Pending() {
		t.Fatal("new tracker should not be pending")
	}
	tracker.Feed("g")
	if !tracker.Pending() {
		t.Fatal("tracker should be pending after feeding 'g'")
	}
	tracker.Feed("g")
	if tracker.Pending() {
		t.Fatal("tracker should not be pending after resolving")
	}
}

func TestGTrackerReset(t *testing.T) {
	var tracker GTracker
	tracker.Feed("g")
	if !tracker.Pending() {
		t.Fatal("expected pending after 'g'")
	}
	tracker.Reset()
	if tracker.Pending() {
		t.Fatal("expected not pending after Reset()")
	}
}

// ---------------------------------------------------------------------------
// truncate (tracklist.go)
// ---------------------------------------------------------------------------

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		maxW  int
		check func(t *testing.T, result string)
	}{
		{
			name:  "short ASCII string unchanged",
			input: "hello",
			maxW:  10,
			check: func(t *testing.T, result string) {
				if result != "hello" {
					t.Errorf("got %q, want %q", result, "hello")
				}
			},
		},
		{
			name:  "exact width unchanged",
			input: "hello",
			maxW:  5,
			check: func(t *testing.T, result string) {
				if result != "hello" {
					t.Errorf("got %q, want %q", result, "hello")
				}
			},
		},
		{
			name:  "ASCII truncated with ellipsis",
			input: "hello world this is long",
			maxW:  10,
			check: func(t *testing.T, result string) {
				w := lipgloss.Width(result)
				if w > 10 {
					t.Errorf("display width %d exceeds maxW 10", w)
				}
				if !strings.HasSuffix(result, "...") {
					t.Errorf("expected '...' suffix, got %q", result)
				}
			},
		},
		{
			name:  "empty string",
			input: "",
			maxW:  10,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("got %q, want empty", result)
				}
			},
		},
		{
			name:  "maxW<=3 no ellipsis",
			input: "hello",
			maxW:  3,
			check: func(t *testing.T, result string) {
				runes := []rune(result)
				if len(runes) > 3 {
					t.Errorf("expected at most 3 runes, got %d: %q", len(runes), result)
				}
			},
		},
		{
			name:  "maxW 1",
			input: "hello",
			maxW:  1,
			check: func(t *testing.T, result string) {
				runes := []rune(result)
				if len(runes) > 1 {
					t.Errorf("expected at most 1 rune, got %d: %q", len(runes), result)
				}
			},
		},
		{
			name:  "CJK characters (double-width)",
			input: "你好世界test",
			maxW:  10,
			check: func(t *testing.T, result string) {
				w := lipgloss.Width(result)
				if w > 10 {
					t.Errorf("display width %d exceeds maxW 10", w)
				}
			},
		},
		{
			name:  "CJK string that fits",
			input: "你好",
			maxW:  10,
			check: func(t *testing.T, result string) {
				if result != "你好" {
					t.Errorf("got %q, want %q", result, "你好")
				}
			},
		},
		{
			name:  "CJK with maxW 3 (fewer runes than maxW)",
			input: "你好",
			maxW:  3,
			check: func(t *testing.T, result string) {
				// "你好" is width 4 with only 2 runes — must not panic
				w := lipgloss.Width(result)
				if w > 3 {
					t.Errorf("display width %d exceeds maxW 3", w)
				}
			},
		},
		{
			name:  "maxW 0 returns empty",
			input: "hello",
			maxW:  0,
			check: func(t *testing.T, result string) {
				if result != "" {
					t.Errorf("got %q, want empty", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncate(tt.input, tt.maxW)
			tt.check(t, result)
		})
	}
}

// ---------------------------------------------------------------------------
// fmtDur (statusbar.go)
// ---------------------------------------------------------------------------

func TestFmtDur(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{"zero", 0, "0:00"},
		{"one minute five seconds", 65 * time.Second, "1:05"},
		{"ten minutes", 10 * time.Minute, "10:00"},
		{"negative clamped to zero", -5 * time.Second, "0:00"},
		{"large value", 90 * time.Minute, "90:00"},
		{"one hour one second", 1*time.Hour + 1*time.Second, "60:01"},
		{"sub-second rounds down", 500 * time.Millisecond, "0:00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := fmtDur(tt.dur)
			if got != tt.want {
				t.Errorf("fmtDur(%v) = %q, want %q", tt.dur, got, tt.want)
			}
		})
	}
}

func TestFmtDurNegativeLarger(t *testing.T) {
	// Extra coverage: negative durations larger than 1 minute should also clamp to 0:00
	got := fmtDur(-65 * time.Second)
	if got != "0:00" {
		t.Errorf("fmtDur(-65s) = %q, want %q", got, "0:00")
	}
}

// ---------------------------------------------------------------------------
// FormatTrackListInfo (tracklist.go)
// ---------------------------------------------------------------------------

func TestFormatTrackListInfo(t *testing.T) {
	tests := []struct {
		name   string
		tracks []source.Track
		want   string
	}{
		{
			name:   "empty list",
			tracks: nil,
			want:   "",
		},
		{
			name: "single track",
			tracks: []source.Track{
				{Name: "Song", Duration: 3 * time.Minute},
			},
			want: "1 track \u00b7 3m",
		},
		{
			name: "multiple tracks under one hour",
			tracks: []source.Track{
				{Name: "A", Duration: 3 * time.Minute},
				{Name: "B", Duration: 4 * time.Minute},
				{Name: "C", Duration: 5 * time.Minute},
			},
			want: "3 tracks \u00b7 12m",
		},
		{
			name: "tracks summing to hours",
			tracks: []source.Track{
				{Name: "A", Duration: 30 * time.Minute},
				{Name: "B", Duration: 45 * time.Minute},
				{Name: "C", Duration: 27 * time.Minute},
			},
			want: "3 tracks \u00b7 1h 42m",
		},
		{
			name: "separators and album rows excluded from count",
			tracks: []source.Track{
				{Name: "---", IsSeparator: true},
				{Name: "A", Duration: 3 * time.Minute},
				{Name: "Album X", IsAlbumRow: true},
				{Name: "B", Duration: 4 * time.Minute},
			},
			want: "2 tracks \u00b7 7m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatTrackListInfo(tt.tracks)
			if got != tt.want {
				t.Errorf("FormatTrackListInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatAlbumInfo (tracklist.go)
// ---------------------------------------------------------------------------

func TestFormatAlbumInfo(t *testing.T) {
	tests := []struct {
		name   string
		artist string
		year   string
		tracks []source.Track
		want   string
	}{
		{
			name:   "artist and year present",
			artist: "Artist",
			year:   "2023",
			tracks: []source.Track{
				{Name: "A"}, {Name: "B"},
			},
			want: "Artist \u00b7 2023 \u00b7 2 tracks",
		},
		{
			name:   "missing artist",
			artist: "",
			year:   "2023",
			tracks: []source.Track{
				{Name: "A"},
			},
			want: "2023 \u00b7 1 track",
		},
		{
			name:   "missing year",
			artist: "Artist",
			year:   "",
			tracks: []source.Track{
				{Name: "A"}, {Name: "B"}, {Name: "C"},
			},
			want: "Artist \u00b7 3 tracks",
		},
		{
			name:   "separators excluded from count",
			artist: "Artist",
			year:   "2023",
			tracks: []source.Track{
				{Name: "---", IsSeparator: true},
				{Name: "A"},
			},
			want: "Artist \u00b7 2023 \u00b7 1 track",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatAlbumInfo(tt.artist, tt.year, tt.tracks)
			if got != tt.want {
				t.Errorf("FormatAlbumInfo() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// FormatPartialTrackListInfo (tracklist.go)
// ---------------------------------------------------------------------------

func TestFormatPartialTrackListInfo(t *testing.T) {
	got := FormatPartialTrackListInfo(50, 200)
	want := "50 of 200 tracks"
	if got != want {
		t.Errorf("FormatPartialTrackListInfo(50, 200) = %q, want %q", got, want)
	}

	got = FormatPartialTrackListInfo(0, 0)
	want = "0 of 0 tracks"
	if got != want {
		t.Errorf("FormatPartialTrackListInfo(0, 0) = %q, want %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// lightenColor (nowplaying.go)
// ---------------------------------------------------------------------------

func TestLightenColor(t *testing.T) {
	tests := []struct {
		name  string
		input lipgloss.Color
		check func(t *testing.T, result lipgloss.Color)
	}{
		{
			name:  "dark color gets lightened",
			input: lipgloss.Color("#1A1A1A"),
			check: func(t *testing.T, result lipgloss.Color) {
				// Should be brighter than input
				if string(result) == "#1A1A1A" || string(result) == "#1a1a1a" {
					t.Error("dark color should have been lightened")
				}
				// Should still be a valid hex color
				if !strings.HasPrefix(string(result), "#") || len(string(result)) != 7 {
					t.Errorf("result %q is not a valid hex color", string(result))
				}
			},
		},
		{
			name:  "bright color stays unchanged",
			input: lipgloss.Color("#FFFFFF"),
			check: func(t *testing.T, result lipgloss.Color) {
				if string(result) != "#FFFFFF" {
					t.Errorf("bright color should be unchanged, got %q", string(result))
				}
			},
		},
		{
			name:  "above threshold stays unchanged",
			input: lipgloss.Color("#C8C8C8"),
			check: func(t *testing.T, result lipgloss.Color) {
				if string(result) != "#C8C8C8" {
					t.Errorf("color above threshold should be unchanged, got %q", string(result))
				}
			},
		},
		{
			name:  "invalid color returns as-is",
			input: lipgloss.Color("not-a-color"),
			check: func(t *testing.T, result lipgloss.Color) {
				if string(result) != "not-a-color" {
					t.Errorf("invalid color should be returned as-is, got %q", string(result))
				}
			},
		},
		{
			name:  "empty string returns as-is",
			input: lipgloss.Color(""),
			check: func(t *testing.T, result lipgloss.Color) {
				if string(result) != "" {
					t.Errorf("empty string should be returned as-is, got %q", string(result))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lightenColor(tt.input)
			tt.check(t, result)
		})
	}
}

// ---------------------------------------------------------------------------
// Toast (toast.go)
// ---------------------------------------------------------------------------

func TestToastShowHideVisible(t *testing.T) {
	var toast Toast
	if toast.Visible() {
		t.Fatal("new toast should not be visible")
	}

	toast.Show("hello", "", ToastSuccess)
	if !toast.Visible() {
		t.Fatal("toast should be visible after Show()")
	}
	if toast.message != "hello" {
		t.Errorf("message = %q, want %q", toast.message, "hello")
	}

	toast.Hide()
	if toast.Visible() {
		t.Fatal("toast should not be visible after Hide()")
	}
}

func TestToastMessageTruncation(t *testing.T) {
	var toast Toast
	longMsg := strings.Repeat("x", 100)
	toast.Show(longMsg, "", ToastInfo)

	if len(toast.message) > 60 {
		t.Errorf("message length %d exceeds 60 char limit", len(toast.message))
	}
	if !strings.HasSuffix(toast.message, "...") {
		t.Errorf("truncated message should end with '...', got %q", toast.message)
	}
}

func TestToastNewlineStripping(t *testing.T) {
	var toast Toast
	toast.Show("line1\nline2", "detail\nmore", ToastError)

	if strings.Contains(toast.message, "\n") {
		t.Errorf("message should not contain newlines: %q", toast.message)
	}
	if strings.Contains(toast.detail, "\n") {
		t.Errorf("detail should not contain newlines: %q", toast.detail)
	}
}

func TestToastCurlyBraceStripping(t *testing.T) {
	var toast Toast
	toast.Show("error{json stuff}", "", ToastError)

	if strings.Contains(toast.message, "{") {
		t.Errorf("message should be truncated at '{': %q", toast.message)
	}
}

func TestToastView(t *testing.T) {
	var toast Toast

	// Not visible => empty
	if got := toast.View(80); got != "" {
		t.Errorf("View() on hidden toast = %q, want empty", got)
	}

	toast.Show("Test message", "some detail", ToastInfo)
	got := toast.View(80)
	if got == "" {
		t.Fatal("View() should return non-empty for visible toast")
	}
}

func TestToastTypes(t *testing.T) {
	var toast Toast

	toast.Show("success", "", ToastSuccess)
	if toast.icon() != "\u2713" {
		t.Errorf("ToastSuccess icon = %q, want checkmark", toast.icon())
	}

	toast.Show("error", "", ToastError)
	if toast.icon() != "\u2717" {
		t.Errorf("ToastError icon = %q, want x-mark", toast.icon())
	}

	toast.Show("info", "", ToastInfo)
	if toast.icon() != "\u266a" {
		t.Errorf("ToastInfo icon = %q, want music note", toast.icon())
	}
}

// ---------------------------------------------------------------------------
// Mode.String() (mode.go)
// ---------------------------------------------------------------------------

func TestModeString(t *testing.T) {
	tests := []struct {
		mode Mode
		want string
	}{
		{ModeNormal, "NORMAL"},
		{ModeCommand, "COMMAND"},
		{ModeSearch, "SEARCH"},
		{ModeFilter, "FILTER"},
		{ModeHelp, "HELP"},
		{ModeActions, "ACTIONS"},
		{ModeDevices, "DEVICES"},
		{ModeNowPlaying, "NOW PLAYING"},
		{Mode(99), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.mode.String()
			if got != tt.want {
				t.Errorf("Mode(%d).String() = %q, want %q", tt.mode, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParseCommand edge cases (command.go)
// ---------------------------------------------------------------------------

func TestParseCommandWhitespace(t *testing.T) {
	// Leading/trailing whitespace should be trimmed
	got, err := ParseCommand("  q  ")
	if err != nil {
		t.Fatalf("ParseCommand with whitespace: %v", err)
	}
	if got.Type != CmdQuit {
		t.Errorf("Type = %v, want CmdQuit", got.Type)
	}
}

func TestParseCommandEmpty(t *testing.T) {
	_, err := ParseCommand("")
	if err == nil {
		t.Fatal("expected error for empty command")
	}
}

func TestParseCommandOnlyWhitespace(t *testing.T) {
	_, err := ParseCommand("   ")
	if err == nil {
		t.Fatal("expected error for whitespace-only command")
	}
}

func TestParseCommandSearchWithArgs(t *testing.T) {
	got, err := ParseCommand("search foo bar baz")
	if err != nil {
		t.Fatalf("ParseCommand('search foo bar baz'): %v", err)
	}
	if got.Type != CmdSearch {
		t.Errorf("Type = %v, want CmdSearch", got.Type)
	}
	if got.StrArg != "foo bar baz" {
		t.Errorf("StrArg = %q, want %q", got.StrArg, "foo bar baz")
	}
}

func TestParseCommandSearchNoArgs(t *testing.T) {
	got, err := ParseCommand("search")
	if err != nil {
		t.Fatalf("ParseCommand('search'): %v", err)
	}
	if got.Type != CmdSearch {
		t.Errorf("Type = %v, want CmdSearch", got.Type)
	}
	if got.StrArg != "" {
		t.Errorf("StrArg = %q, want empty", got.StrArg)
	}
}

func TestParseCommandVolNegative(t *testing.T) {
	_, err := ParseCommand("vol -5")
	if err == nil {
		t.Fatal("expected error for negative volume")
	}
}

func TestParseCommandVolNoArg(t *testing.T) {
	_, err := ParseCommand("vol")
	if err == nil {
		t.Fatal("expected error for vol without argument")
	}
}

func TestParseCommandRepeatInvalidMode(t *testing.T) {
	_, err := ParseCommand("repeat invalid")
	if err == nil {
		t.Fatal("expected error for invalid repeat mode")
	}
}

func TestParseCommandRepeatNoArg(t *testing.T) {
	_, err := ParseCommand("repeat")
	if err == nil {
		t.Fatal("expected error for repeat without argument")
	}
}

// ---------------------------------------------------------------------------
// rgbHex (albumart.go)
// ---------------------------------------------------------------------------

func TestRgbHex(t *testing.T) {
	tests := []struct {
		r, g, b uint8
		want    string
	}{
		{0, 0, 0, "#000000"},
		{255, 255, 255, "#ffffff"},
		{0x1d, 0xb9, 0x54, "#1db954"},
		{255, 0, 128, "#ff0080"},
		{1, 2, 3, "#010203"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := rgbHex(tt.r, tt.g, tt.b)
			if got != tt.want {
				t.Errorf("rgbHex(%d, %d, %d) = %q, want %q", tt.r, tt.g, tt.b, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// backoffInterval (app.go)
// ---------------------------------------------------------------------------

func TestBackoffInterval(t *testing.T) {
	tests := []struct {
		name              string
		consecutiveErrors int
		wantMultiplier    int
	}{
		{"zero errors returns base interval", 0, 1},
		{"1 error returns 2x", 1, 2},
		{"2 errors returns 4x", 2, 4},
		{"3 errors returns 8x", 3, 8},
		{"5 errors returns 32x (cap)", 5, 32},
		{"10 errors still capped at 32x", 10, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{consecutiveErrors: tt.consecutiveErrors}
			got := m.backoffInterval()
			want := pollInterval * time.Duration(tt.wantMultiplier)
			if got != want {
				t.Errorf("backoffInterval() with %d errors = %v, want %v", tt.consecutiveErrors, got, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// pluralize helper (tracklist.go)
// ---------------------------------------------------------------------------

func TestPluralize(t *testing.T) {
	if got := pluralize(1, "track", "tracks"); got != "track" {
		t.Errorf("pluralize(1) = %q, want %q", got, "track")
	}
	if got := pluralize(0, "track", "tracks"); got != "tracks" {
		t.Errorf("pluralize(0) = %q, want %q", got, "tracks")
	}
	if got := pluralize(5, "track", "tracks"); got != "tracks" {
		t.Errorf("pluralize(5) = %q, want %q", got, "tracks")
	}
}

// ---------------------------------------------------------------------------
// renderHalfBlocks edge cases (albumart.go)
// ---------------------------------------------------------------------------

func TestRenderHalfBlocksZeroDimensions(t *testing.T) {
	// A nil-safe check: zero width/height should return empty string without panic.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	if got := renderHalfBlocks(img, 0, 0); got != "" {
		t.Errorf("renderHalfBlocks(img, 0, 0) = %q, want empty", got)
	}
	if got := renderHalfBlocks(img, 0, 5); got != "" {
		t.Errorf("renderHalfBlocks(img, 0, 5) = %q, want empty", got)
	}
	if got := renderHalfBlocks(img, 5, 0); got != "" {
		t.Errorf("renderHalfBlocks(img, 5, 0) = %q, want empty", got)
	}
}

func TestRenderHalfBlocksSmallImage(t *testing.T) {
	// 4x4 solid red image should produce non-empty half-block output.
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	red := color.RGBA{R: 255, A: 255}
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, red)
		}
	}
	got := renderHalfBlocks(img, 4, 2)
	if got == "" {
		t.Fatal("renderHalfBlocks with 4x4 image should return non-empty string")
	}
	if !strings.Contains(got, "▀") {
		t.Error("output should contain half-block character '▀'")
	}
}

// ---------------------------------------------------------------------------
// PlaceholderArt edge cases (albumart.go)
// ---------------------------------------------------------------------------

func TestPlaceholderArtZero(t *testing.T) {
	got := PlaceholderArt(0, 0)
	if got != "" {
		t.Errorf("PlaceholderArt(0, 0) = %q, want empty", got)
	}
}

func TestPlaceholderArtSmall(t *testing.T) {
	got := PlaceholderArt(3, 2)
	if got == "" {
		t.Fatal("PlaceholderArt(3, 2) should return non-empty string")
	}
	if !strings.Contains(got, "▀") {
		t.Error("PlaceholderArt output should contain half-block character '▀'")
	}
}

// ---------------------------------------------------------------------------
// DominantColor edge cases (albumart.go)
// ---------------------------------------------------------------------------

func TestDominantColorZeroBounds(t *testing.T) {
	// An image with zero-size bounds should return "" without panic.
	img := image.NewRGBA(image.Rect(0, 0, 0, 0))
	got := DominantColor(img)
	if got != "" {
		t.Errorf("DominantColor(zero-bounds) = %q, want empty", got)
	}
}

func TestDominantColorSolidColor(t *testing.T) {
	// A solid mid-saturation color image should return a valid hex color.
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	c := color.RGBA{R: 0, G: 150, B: 200, A: 255}
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			img.Set(x, y, c)
		}
	}
	got := DominantColor(img)
	if got == "" {
		t.Fatal("DominantColor with a saturated solid color should return non-empty")
	}
	if !strings.HasPrefix(string(got), "#") || len(string(got)) != 7 {
		t.Errorf("DominantColor returned invalid hex color: %q", string(got))
	}
}

// ---------------------------------------------------------------------------
// RenderNowPlaying (nowplaying.go)
// ---------------------------------------------------------------------------

func TestRenderNowPlayingNilTrack(t *testing.T) {
	// Should not panic and should contain "No track playing".
	got := RenderNowPlaying(nil, "", nil, false, 0, 80, 40)
	if got == "" {
		t.Fatal("RenderNowPlaying with nil track should return non-empty")
	}
	if !strings.Contains(got, "No track playing") {
		t.Error("output should contain 'No track playing' when track is nil")
	}
}

func TestRenderNowPlayingValidTrack(t *testing.T) {
	track := &source.Track{
		Name:     "Test Song",
		Artist:   "Test Artist",
		Album:    "Test Album",
		Duration: 3 * time.Minute,
		Position: 1 * time.Minute,
	}
	got := RenderNowPlaying(track, "", nil, false, 0, 80, 40)
	if got == "" {
		t.Fatal("RenderNowPlaying with valid track should return non-empty")
	}
}

func TestRenderNowPlayingZeroSize(t *testing.T) {
	// Zero terminal size should return empty.
	got := RenderNowPlaying(nil, "", nil, false, 0, 0, 0)
	if got != "" {
		t.Errorf("RenderNowPlaying with zero size = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// Pane.String() (mode.go)
// ---------------------------------------------------------------------------

func TestPaneString(t *testing.T) {
	tests := []struct {
		pane Pane
		want string
	}{
		{PaneSidebar, "SIDEBAR"},
		{PaneTrackList, "TRACKLIST"},
		{Pane(99), ""},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.pane.String()
			if got != tt.want {
				t.Errorf("Pane(%d).String() = %q, want %q", tt.pane, got, tt.want)
			}
		})
	}
}
