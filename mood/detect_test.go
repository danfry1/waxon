package mood

import "testing"

func TestDetectMood(t *testing.T) {
	tests := []struct {
		name, artist, track, album, wantMood string
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
				t.Errorf("DetectMood(%q, %q, %q) = %q, want %q", tt.artist, tt.track, tt.album, got.Name, tt.wantMood)
			}
		})
	}
}
