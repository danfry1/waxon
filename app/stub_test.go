package app

import (
	"context"
	"time"

	"github.com/danfry1/waxon/source"
)

// StubSource is a test double for source.RichSource. Each method delegates to
// a function field when set, or returns zero values. This lets individual
// tests configure only the methods they care about.
type StubSource struct {
	CurrentPlaybackFn    func(context.Context) (*source.PlaybackState, error)
	PlayFn               func(context.Context) error
	PauseFn              func(context.Context) error
	NextFn               func(context.Context) error
	PreviousFn           func(context.Context) error
	SeekFn               func(context.Context, time.Duration) error
	QueueFn              func(context.Context) ([]source.Track, error)
	PlaylistsFn          func(context.Context) ([]source.Playlist, error)
	PlaylistTracksPageFn func(context.Context, string, int, int) ([]source.Track, int, error)
	LikedTracksPageFn    func(context.Context, int, int) ([]source.Track, int, error)
	RecentlyPlayedFn     func(context.Context) ([]source.Track, error)
	SearchFn             func(context.Context, string) (*source.SearchResults, error)
	SetVolumeFn          func(context.Context, int) error
	DevicesFn            func(context.Context) ([]source.Device, error)
	TransferPlaybackFn   func(context.Context, string) error
	SetShuffleFn         func(context.Context, bool) error
	SetRepeatFn          func(context.Context, source.RepeatMode) error
	AudioFeaturesFn      func(context.Context, string) (*source.AudioFeatures, error)
	PlayTrackFn          func(context.Context, string, string) error
	PlayTrackDirectFn    func(context.Context, string) error
	AddToQueueFn         func(context.Context, string) error
	GetArtistFn          func(context.Context, string) (*source.ArtistPage, error)
	GetAlbumFn           func(context.Context, string) (*source.AlbumPage, error)
}

func (s *StubSource) CurrentPlayback(ctx context.Context) (*source.PlaybackState, error) {
	if s.CurrentPlaybackFn != nil {
		return s.CurrentPlaybackFn(ctx)
	}
	return nil, nil
}

func (s *StubSource) Play(ctx context.Context) error {
	if s.PlayFn != nil {
		return s.PlayFn(ctx)
	}
	return nil
}

func (s *StubSource) Pause(ctx context.Context) error {
	if s.PauseFn != nil {
		return s.PauseFn(ctx)
	}
	return nil
}

func (s *StubSource) Next(ctx context.Context) error {
	if s.NextFn != nil {
		return s.NextFn(ctx)
	}
	return nil
}

func (s *StubSource) Previous(ctx context.Context) error {
	if s.PreviousFn != nil {
		return s.PreviousFn(ctx)
	}
	return nil
}

func (s *StubSource) Seek(ctx context.Context, pos time.Duration) error {
	if s.SeekFn != nil {
		return s.SeekFn(ctx, pos)
	}
	return nil
}

func (s *StubSource) Queue(ctx context.Context) ([]source.Track, error) {
	if s.QueueFn != nil {
		return s.QueueFn(ctx)
	}
	return nil, nil
}

func (s *StubSource) Playlists(ctx context.Context) ([]source.Playlist, error) {
	if s.PlaylistsFn != nil {
		return s.PlaylistsFn(ctx)
	}
	return nil, nil
}

func (s *StubSource) PlaylistTracksPage(ctx context.Context, id string, offset, limit int) ([]source.Track, int, error) {
	if s.PlaylistTracksPageFn != nil {
		return s.PlaylistTracksPageFn(ctx, id, offset, limit)
	}
	return nil, 0, nil
}

func (s *StubSource) LikedTracksPage(ctx context.Context, offset, limit int) ([]source.Track, int, error) {
	if s.LikedTracksPageFn != nil {
		return s.LikedTracksPageFn(ctx, offset, limit)
	}
	return nil, 0, nil
}

func (s *StubSource) RecentlyPlayed(ctx context.Context) ([]source.Track, error) {
	if s.RecentlyPlayedFn != nil {
		return s.RecentlyPlayedFn(ctx)
	}
	return nil, nil
}

func (s *StubSource) Search(ctx context.Context, query string) (*source.SearchResults, error) {
	if s.SearchFn != nil {
		return s.SearchFn(ctx, query)
	}
	return &source.SearchResults{}, nil
}

func (s *StubSource) SetVolume(ctx context.Context, percent int) error {
	if s.SetVolumeFn != nil {
		return s.SetVolumeFn(ctx, percent)
	}
	return nil
}

func (s *StubSource) Devices(ctx context.Context) ([]source.Device, error) {
	if s.DevicesFn != nil {
		return s.DevicesFn(ctx)
	}
	return nil, nil
}

func (s *StubSource) TransferPlayback(ctx context.Context, deviceID string) error {
	if s.TransferPlaybackFn != nil {
		return s.TransferPlaybackFn(ctx, deviceID)
	}
	return nil
}

func (s *StubSource) SetShuffle(ctx context.Context, state bool) error {
	if s.SetShuffleFn != nil {
		return s.SetShuffleFn(ctx, state)
	}
	return nil
}

func (s *StubSource) SetRepeat(ctx context.Context, mode source.RepeatMode) error {
	if s.SetRepeatFn != nil {
		return s.SetRepeatFn(ctx, mode)
	}
	return nil
}

func (s *StubSource) AudioFeatures(ctx context.Context, trackID string) (*source.AudioFeatures, error) {
	if s.AudioFeaturesFn != nil {
		return s.AudioFeaturesFn(ctx, trackID)
	}
	return nil, nil
}

func (s *StubSource) PlayTrack(ctx context.Context, contextURI, trackURI string) error {
	if s.PlayTrackFn != nil {
		return s.PlayTrackFn(ctx, contextURI, trackURI)
	}
	return nil
}

func (s *StubSource) PlayTrackDirect(ctx context.Context, trackURI string) error {
	if s.PlayTrackDirectFn != nil {
		return s.PlayTrackDirectFn(ctx, trackURI)
	}
	return nil
}

func (s *StubSource) AddToQueue(ctx context.Context, trackID string) error {
	if s.AddToQueueFn != nil {
		return s.AddToQueueFn(ctx, trackID)
	}
	return nil
}

func (s *StubSource) GetArtist(ctx context.Context, artistID string) (*source.ArtistPage, error) {
	if s.GetArtistFn != nil {
		return s.GetArtistFn(ctx, artistID)
	}
	return nil, nil
}

func (s *StubSource) GetAlbum(ctx context.Context, albumID string) (*source.AlbumPage, error) {
	if s.GetAlbumFn != nil {
		return s.GetAlbumFn(ctx, albumID)
	}
	return nil, nil
}

// Compile-time check that StubSource satisfies RichSource.
var _ source.RichSource = (*StubSource)(nil)

// newTestModel creates a Model with a StubSource and reasonable defaults for testing.
func newTestModel(stub *StubSource) Model {
	m := NewModel(stub)
	m.width = 120
	m.height = 40
	m.layoutResize()
	return m
}
