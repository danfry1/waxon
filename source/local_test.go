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
