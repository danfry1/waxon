package spotify

import (
	"context"
	"time"

	"github.com/danielfry/spotui/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

func (p *PlayerSource) Playlists() ([]source.Playlist, error) {
	ctx := context.Background()
	page, err := p.client.CurrentUsersPlaylists(ctx, spotifyapi.Limit(50))
	if err != nil {
		return nil, err
	}
	playlists := make([]source.Playlist, len(page.Playlists))
	for i, pl := range page.Playlists {
		playlists[i] = source.Playlist{
			ID:         string(pl.ID),
			Name:       pl.Name,
			TrackCount: int(pl.Tracks.Total),
		}
	}
	return playlists, nil
}

func (p *PlayerSource) PlaylistTracks(id string) ([]source.Track, error) {
	ctx := context.Background()
	page, err := p.client.GetPlaylistItems(ctx, spotifyapi.ID(id), spotifyapi.Limit(50))
	if err != nil {
		return nil, err
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, item := range page.Items {
		t := item.Track.Track
		if t == nil {
			continue
		}
		artist := ""
		if len(t.Artists) > 0 {
			artist = t.Artists[0].Name
		}
		artworkURL := ""
		if len(t.Album.Images) > 0 {
			artworkURL = t.Album.Images[0].URL
		}
		tracks = append(tracks, source.Track{
			ID:         string(t.ID),
			Name:       t.Name,
			Artist:     artist,
			Album:      t.Album.Name,
			ArtworkURL: artworkURL,
			Duration:   time.Duration(t.Duration) * time.Millisecond,
		})
	}
	return tracks, nil
}

func (p *PlayerSource) Search(query string) (*source.SearchResults, error) {
	ctx := context.Background()
	result, err := p.client.Search(ctx, query, spotifyapi.SearchTypeTrack|spotifyapi.SearchTypeArtist|spotifyapi.SearchTypeAlbum, spotifyapi.Limit(10))
	if err != nil {
		return nil, err
	}

	sr := &source.SearchResults{}

	if result.Tracks != nil {
		for _, t := range result.Tracks.Tracks {
			artist := ""
			if len(t.Artists) > 0 {
				artist = t.Artists[0].Name
			}
			artworkURL := ""
			if len(t.Album.Images) > 0 {
				artworkURL = t.Album.Images[0].URL
			}
			sr.Tracks = append(sr.Tracks, source.Track{
				ID:         string(t.ID),
				Name:       t.Name,
				Artist:     artist,
				Album:      t.Album.Name,
				ArtworkURL: artworkURL,
				Duration:   time.Duration(t.Duration) * time.Millisecond,
			})
		}
	}
	if result.Artists != nil {
		for _, a := range result.Artists.Artists {
			sr.Artists = append(sr.Artists, a.Name)
		}
	}
	if result.Albums != nil {
		for _, a := range result.Albums.Albums {
			sr.Albums = append(sr.Albums, a.Name)
		}
	}

	return sr, nil
}
