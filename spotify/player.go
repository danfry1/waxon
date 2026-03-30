package spotify

import (
	"context"
	"fmt"
	"time"

	"github.com/danielfry/spotui/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

type PlayerSource struct {
	client   *spotifyapi.Client
	features *FeatureCache
}

func NewPlayerSource(client *spotifyapi.Client) *PlayerSource {
	return &PlayerSource{
		client:   client,
		features: NewFeatureCache(client),
	}
}

func (p *PlayerSource) CurrentTrack() (*source.Track, error) {
	ctx := context.Background()
	state, err := p.client.PlayerState(ctx)
	if err != nil {
		return nil, fmt.Errorf("player state: %w", err)
	}
	if state == nil || state.Item == nil {
		return nil, nil
	}

	item := state.Item
	artist := ""
	if len(item.Artists) > 0 {
		artist = item.Artists[0].Name
	}

	artworkURL := ""
	if len(item.Album.Images) > 0 {
		artworkURL = item.Album.Images[0].URL
	}

	return &source.Track{
		ID:         string(item.ID),
		Name:       item.Name,
		Artist:     artist,
		Album:      item.Album.Name,
		ArtworkURL: artworkURL,
		Duration:   time.Duration(item.Duration) * time.Millisecond,
		Position:   time.Duration(state.Progress) * time.Millisecond,
		Playing:    state.Playing,
	}, nil
}

func (p *PlayerSource) Play() error {
	return p.client.Play(context.Background())
}

func (p *PlayerSource) Pause() error {
	return p.client.Pause(context.Background())
}

func (p *PlayerSource) Next() error {
	return p.client.Next(context.Background())
}

func (p *PlayerSource) Previous() error {
	return p.client.Previous(context.Background())
}

func (p *PlayerSource) Seek(position time.Duration) error {
	ms := int(position.Milliseconds())
	return p.client.Seek(context.Background(), ms)
}

func (p *PlayerSource) SetVolume(percent int) error {
	return p.client.Volume(context.Background(), percent)
}

func (p *PlayerSource) SetShuffle(state bool) error {
	return p.client.Shuffle(context.Background(), state)
}

func (p *PlayerSource) SetRepeat(mode source.RepeatMode) error {
	return p.client.Repeat(context.Background(), string(mode))
}

func (p *PlayerSource) Devices() ([]source.Device, error) {
	devs, err := p.client.PlayerDevices(context.Background())
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

func (p *PlayerSource) TransferPlayback(deviceID string) error {
	id := spotifyapi.ID(deviceID)
	return p.client.TransferPlayback(context.Background(), id, true)
}

func (p *PlayerSource) Queue() ([]source.Track, error) {
	q, err := p.client.GetQueue(context.Background())
	if err != nil {
		return nil, err
	}
	tracks := make([]source.Track, len(q.Items))
	for i, item := range q.Items {
		artist := ""
		if len(item.Artists) > 0 {
			artist = item.Artists[0].Name
		}
		artworkURL := ""
		if len(item.Album.Images) > 0 {
			artworkURL = item.Album.Images[0].URL
		}
		tracks[i] = source.Track{
			ID:         string(item.ID),
			Name:       item.Name,
			Artist:     artist,
			Album:      item.Album.Name,
			ArtworkURL: artworkURL,
			Duration:   time.Duration(item.Duration) * time.Millisecond,
		}
	}
	return tracks, nil
}

func (p *PlayerSource) AudioFeatures(trackID string) (*source.AudioFeatures, error) {
	return p.features.Get(trackID)
}
