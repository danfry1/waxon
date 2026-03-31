package spotify

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielfry/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

// fullTrackToSource converts a zmb3 FullTrack to a source.Track.
func fullTrackToSource(item spotifyapi.FullTrack) source.Track {
	artist := ""
	artistID := ""
	if len(item.Artists) > 0 {
		artist = item.Artists[0].Name
		artistID = string(item.Artists[0].ID)
	}
	artworkURL := ""
	if len(item.Album.Images) > 0 {
		artworkURL = item.Album.Images[0].URL
	}
	return source.Track{
		ID:         string(item.ID),
		URI:        string(item.URI),
		Name:       item.Name,
		Artist:     artist,
		Album:      item.Album.Name,
		ArtworkURL: artworkURL,
		Duration:   time.Duration(item.Duration) * time.Millisecond,
		ArtistID:   artistID,
		AlbumID:    string(item.Album.ID),
	}
}

type PlayerSource struct {
	client     *spotifyapi.Client
	httpClient *http.Client
	features   *FeatureCache
}

func NewPlayerSource(cp ClientPair) *PlayerSource {
	return &PlayerSource{
		client:     cp.Spotify,
		httpClient: cp.HTTP,
		features:   NewFeatureCache(cp.Spotify),
	}
}

func (p *PlayerSource) CurrentPlayback(ctx context.Context) (*source.PlaybackState, error) {
	state, err := p.client.PlayerState(ctx)
	if err != nil {
		return nil, fmt.Errorf("player state: %w", err)
	}
	if state == nil || state.Item == nil {
		return nil, nil
	}

	track := fullTrackToSource(*state.Item)
	track.Position = time.Duration(state.Progress) * time.Millisecond
	track.Playing = state.Playing
	if state.Device.Name != "" {
		track.DeviceName = state.Device.Name
	}

	repeatMode := source.RepeatOff
	switch state.RepeatState {
	case "context":
		repeatMode = source.RepeatContext
	case "track":
		repeatMode = source.RepeatTrack
	}

	return &source.PlaybackState{
		Track:      &track,
		Volume:     int(state.Device.Volume),
		ShuffleOn:  state.ShuffleState,
		RepeatMode: repeatMode,
	}, nil
}

func (p *PlayerSource) Play(ctx context.Context) error {
	return p.client.Play(ctx)
}

func (p *PlayerSource) Pause(ctx context.Context) error {
	return p.client.Pause(ctx)
}

func (p *PlayerSource) Next(ctx context.Context) error {
	return p.client.Next(ctx)
}

func (p *PlayerSource) Previous(ctx context.Context) error {
	return p.client.Previous(ctx)
}

func (p *PlayerSource) Seek(ctx context.Context, position time.Duration) error {
	ms := int(position.Milliseconds())
	return p.client.Seek(ctx, ms)
}

func (p *PlayerSource) SetVolume(ctx context.Context, percent int) error {
	return p.client.Volume(ctx, percent)
}

func (p *PlayerSource) SetShuffle(ctx context.Context, state bool) error {
	return p.client.Shuffle(ctx, state)
}

func (p *PlayerSource) SetRepeat(ctx context.Context, mode source.RepeatMode) error {
	return p.client.Repeat(ctx, string(mode))
}

func (p *PlayerSource) Devices(ctx context.Context) ([]source.Device, error) {
	devs, err := p.client.PlayerDevices(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]source.Device, len(devs))
	for i, d := range devs {
		result[i] = source.Device{
			ID:       string(d.ID),
			Name:     d.Name,
			Type:     d.Type,
			IsActive: d.Active,
		}
	}
	return result, nil
}

func (p *PlayerSource) TransferPlayback(ctx context.Context, deviceID string) error {
	id := spotifyapi.ID(deviceID)
	return p.client.TransferPlayback(ctx, id, true)
}

func (p *PlayerSource) Queue(ctx context.Context) ([]source.Track, error) {
	q, err := p.client.GetQueue(ctx)
	if err != nil {
		return nil, err
	}
	tracks := make([]source.Track, len(q.Items))
	for i, item := range q.Items {
		tracks[i] = fullTrackToSource(item)
	}
	return tracks, nil
}

func (p *PlayerSource) PlayTrack(ctx context.Context, contextURI string, trackURI string) error {
	uri := spotifyapi.URI(contextURI)
	opts := &spotifyapi.PlayOptions{
		PlaybackContext: &uri,
	}
	if trackURI != "" {
		opts.PlaybackOffset = &spotifyapi.PlaybackOffset{URI: spotifyapi.URI(trackURI)}
	}
	return p.client.PlayOpt(ctx, opts)
}

func (p *PlayerSource) PlayTrackDirect(ctx context.Context, trackURI string) error {
	opts := &spotifyapi.PlayOptions{
		URIs: []spotifyapi.URI{spotifyapi.URI(trackURI)},
	}
	return p.client.PlayOpt(ctx, opts)
}

func (p *PlayerSource) AddToQueue(ctx context.Context, trackID string) error {
	return p.client.QueueSong(ctx, spotifyapi.ID(trackID))
}

func (p *PlayerSource) RecentlyPlayed(ctx context.Context) ([]source.Track, error) {
	var page struct {
		Items []struct {
			Track apiTrack `json:"track"`
		} `json:"items"`
	}
	if err := p.apiGet(ctx, "/me/player/recently-played?limit=50", &page); err != nil {
		return nil, fmt.Errorf("recently played: %w", err)
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, item := range page.Items {
		tracks = append(tracks, apiTrackToSource(item.Track))
	}
	return tracks, nil
}

func (p *PlayerSource) AudioFeatures(ctx context.Context, trackID string) (*source.AudioFeatures, error) {
	return p.features.Get(ctx, trackID)
}
