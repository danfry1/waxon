package source

import "time"

type Track struct {
	Name       string
	Artist     string
	Album      string
	ArtworkURL string
	Duration   time.Duration
	Position   time.Duration
	Playing    bool
}

type TrackSource interface {
	CurrentTrack() (*Track, error)
	Play() error
	Pause() error
	Next() error
	Previous() error
}
