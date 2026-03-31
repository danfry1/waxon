package source

import (
	"context"
	"time"
)

// Track represents a Spotify track or, when IsSeparator/IsAlbumRow is set, a
// display-only row in the track list. The boolean flags create a lightweight
// tagged-union pattern — this is a pragmatic trade-off: Go lacks sum types,
// and an interface+type-switch approach would add verbosity without benefit
// at this project's scale.
type Track struct {
	ID          string
	URI         string
	Name        string
	Artist      string
	Album       string
	ArtworkURL  string
	Duration    time.Duration
	Position    time.Duration
	Playing     bool
	DeviceName  string // active playback device (populated by CurrentPlayback)
	ArtistID    string // first artist's Spotify ID for navigation
	AlbumID     string // album's Spotify ID for navigation
	IsAlbumRow  bool   // true for discography album rows (not playable tracks)
	IsSeparator bool   // true for visual separator rows (not selectable)
}

// PlaybackState bundles the current track with device-level playback state
// so the UI can stay in sync when controlled from other clients.
type PlaybackState struct {
	Track      *Track
	Volume     int
	ShuffleOn  bool
	RepeatMode RepeatMode
}

type TrackSource interface {
	CurrentPlayback(ctx context.Context) (*PlaybackState, error)
	Play(ctx context.Context) error
	Pause(ctx context.Context) error
	Next(ctx context.Context) error
	Previous(ctx context.Context) error
	Seek(ctx context.Context, position time.Duration) error
}
