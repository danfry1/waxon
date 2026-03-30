package mood

import (
	"testing"

	"github.com/danielfry/spotui/source"
)

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

func TestDetectFromFeatures(t *testing.T) {
	tests := []struct {
		name     string
		features source.AudioFeatures
		want     string
	}{
		{"high energy dance → electric", source.AudioFeatures{Energy: 0.85, Valence: 0.6, Danceability: 0.8, Tempo: 128}, "electric"},
		{"low energy ambient → drift", source.AudioFeatures{Energy: 0.15, Valence: 0.3, Danceability: 0.2, Tempo: 80, Acousticness: 0.7}, "drift"},
		{"happy upbeat → bright", source.AudioFeatures{Energy: 0.7, Valence: 0.8, Danceability: 0.7, Tempo: 120}, "bright"},
		{"angry intense → dark", source.AudioFeatures{Energy: 0.8, Valence: 0.15, Danceability: 0.4, Tempo: 140}, "dark"},
		{"acoustic mellow → warm", source.AudioFeatures{Energy: 0.35, Valence: 0.5, Danceability: 0.3, Tempo: 95, Acousticness: 0.8}, "warm"},
		{"soulful groove → golden", source.AudioFeatures{Energy: 0.45, Valence: 0.65, Danceability: 0.6, Tempo: 110}, "golden"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectFromFeatures(&tt.features)
			if got.Name != tt.want {
				t.Errorf("DetectFromFeatures() = %q, want %q", got.Name, tt.want)
			}
		})
	}
}

func TestDetectFromFeaturesNil(t *testing.T) {
	got := DetectFromFeatures(nil)
	if got.Name != Warm.Name {
		t.Errorf("nil features should default to warm, got %q", got.Name)
	}
}
