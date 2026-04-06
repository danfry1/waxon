package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

// LikedPlaylistID is the sentinel playlist ID for the user's Liked Songs.
const LikedPlaylistID = "liked"

const apiBase = "https://api.spotify.com/v1"

// Spotify API response types (the zmb3 library uses "tracks" but the API
// now returns "items" for the playlist track collection).

type apiPlaylistPage struct {
	Items []apiSimplePlaylist `json:"items"`
}

type apiSimplePlaylist struct {
	ID     string `json:"id"`
	URI    string `json:"uri"`
	Name   string `json:"name"`
	Images []struct {
		URL string `json:"url"`
	} `json:"images"`
	Items struct {
		Total int `json:"total"`
	} `json:"items"`
	// Fallback for older API format
	Tracks struct {
		Total int `json:"total"`
	} `json:"tracks"`
}

type apiFullPlaylist struct {
	Name  string `json:"name"`
	Items struct {
		Items []apiPlaylistItem `json:"items"`
		Total int               `json:"total"`
	} `json:"items"`
}

type apiPlaylistItem struct {
	Item apiTrack `json:"item"`
}

type apiTrack struct {
	ID       string `json:"id"`
	URI      string `json:"uri"`
	Name     string `json:"name"`
	Duration int    `json:"duration_ms"`
	Album    struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	} `json:"album"`
	Artists []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"artists"`
}

// apiTrackToSource converts an apiTrack to a source.Track.
func apiTrackToSource(t apiTrack) source.Track {
	artist := ""
	artistID := ""
	if len(t.Artists) > 0 {
		artist = t.Artists[0].Name
		artistID = t.Artists[0].ID
	}
	artworkURL := ""
	if len(t.Album.Images) > 0 {
		artworkURL = t.Album.Images[0].URL
	}
	return source.Track{
		ID:         t.ID,
		URI:        t.URI,
		Name:       t.Name,
		Artist:     artist,
		Album:      t.Album.Name,
		ArtworkURL: artworkURL,
		Duration:   time.Duration(t.Duration) * time.Millisecond,
		ArtistID:   artistID,
		AlbumID:    t.Album.ID,
	}
}

// maxAPIResponseBytes limits API response body reads to 5 MB.
const maxAPIResponseBytes = 5 << 20

// APIError is returned by apiGet for non-200 responses, allowing callers
// to inspect the HTTP status code via errors.As.
type APIError struct {
	StatusCode int
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("spotify API %s: %d %s", e.Path, e.StatusCode, e.Body)
}

func (e *APIError) HTTPStatus() int {
	return e.StatusCode
}

func (p *PlayerSource) apiGet(ctx context.Context, path string, v any) error {
	return p.apiGetRetry(ctx, path, v, 0)
}

const maxRetries = 2

func (p *PlayerSource) apiGetRetry(ctx context.Context, path string, v any, attempt int) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBase+path, nil)
	if err != nil {
		return err
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxAPIResponseBytes)

	// Handle 429 Too Many Requests with Retry-After
	if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
		_, _ = io.ReadAll(limited) // drain body
		wait := 1
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if parsed, err := strconv.Atoi(ra); err == nil && parsed > 0 && parsed <= 10 {
				wait = parsed
			}
		}
		slog.Warn("rate limited by Spotify API, retrying", "path", path, "wait_seconds", wait, "attempt", attempt+1)
		select {
		case <-time.After(time.Duration(wait) * time.Second):
		case <-ctx.Done():
			return ctx.Err()
		}
		return p.apiGetRetry(ctx, path, v, attempt+1)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(limited)
		slog.Debug("API error", "path", path, "status", resp.StatusCode)
		return &APIError{StatusCode: resp.StatusCode, Path: path, Body: string(body)}
	}
	return json.NewDecoder(limited).Decode(v)
}

func (p *PlayerSource) Playlists(ctx context.Context) ([]source.Playlist, error) {
	// Fetch liked songs count (non-fatal on failure — show 0 count)
	var likedPage struct {
		Total int `json:"total"`
	}
	if err := p.apiGet(ctx, "/me/tracks?limit=1", &likedPage); err != nil {
		slog.Warn("failed to fetch liked songs count", "error", err)
	}

	liked := source.Playlist{
		ID:         LikedPlaylistID,
		URI:        "spotify:collection:tracks",
		Name:       "♥ Liked Songs",
		TrackCount: likedPage.Total,
	}

	playlists := []source.Playlist{liked}

	// Paginate through all user playlists (Spotify API returns max 50 per page)
	offset := 0
	for {
		var page apiPlaylistPage
		path := fmt.Sprintf("/me/playlists?limit=50&offset=%d", offset)
		if err := p.apiGet(ctx, path, &page); err != nil {
			return nil, err
		}
		if len(page.Items) == 0 {
			break
		}
		for _, pl := range page.Items {
			total := pl.Items.Total
			if total == 0 {
				total = pl.Tracks.Total // fallback for older API
			}
			imageURL := ""
			if len(pl.Images) > 0 {
				imageURL = pl.Images[0].URL
			}
			playlists = append(playlists, source.Playlist{
				ID:         pl.ID,
				URI:        pl.URI,
				Name:       pl.Name,
				ImageURL:   imageURL,
				TrackCount: total,
			})
		}
		offset += len(page.Items)
		if len(page.Items) < 50 {
			break // last page
		}
	}
	return playlists, nil
}

func (p *PlayerSource) PlaylistTracksPage(ctx context.Context, id string, offset, limit int) ([]source.Track, int, error) {
	if id == LikedPlaylistID {
		return p.LikedTracksPage(ctx, offset, limit)
	}

	if offset == 0 {
		var full apiFullPlaylist
		if err := p.apiGet(ctx, "/playlists/"+id, &full); err != nil {
			return nil, 0, err
		}
		tracks := make([]source.Track, 0, len(full.Items.Items))
		for _, item := range full.Items.Items {
			tracks = append(tracks, apiTrackToSource(item.Item))
		}
		return tracks, full.Items.Total, nil
	}

	var page struct {
		Items []apiPlaylistItem `json:"items"`
		Total int               `json:"total"`
	}
	path := fmt.Sprintf("/playlists/%s/tracks?offset=%d&limit=%d", id, offset, limit)
	if err := p.apiGet(ctx, path, &page); err != nil {
		return nil, 0, err
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, item := range page.Items {
		tracks = append(tracks, apiTrackToSource(item.Item))
	}
	return tracks, page.Total, nil
}

func (p *PlayerSource) LikedTracksPage(ctx context.Context, offset, limit int) ([]source.Track, int, error) {
	if limit > 50 {
		limit = 50
	}
	var page struct {
		Items []struct {
			Track apiTrack `json:"track"`
		} `json:"items"`
		Total int `json:"total"`
	}
	path := fmt.Sprintf("/me/tracks?limit=%d&offset=%d", limit, offset)
	if err := p.apiGet(ctx, path, &page); err != nil {
		return nil, 0, err
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, item := range page.Items {
		tracks = append(tracks, apiTrackToSource(item.Track))
	}
	return tracks, page.Total, nil
}

func (p *PlayerSource) Search(ctx context.Context, query string) (*source.SearchResults, error) {
	result, err := p.client.Search(ctx, query, spotifyapi.SearchTypeTrack|spotifyapi.SearchTypeArtist|spotifyapi.SearchTypeAlbum, spotifyapi.Limit(10))
	if err != nil {
		return nil, err
	}

	sr := &source.SearchResults{}

	if result.Tracks != nil {
		for _, t := range result.Tracks.Tracks {
			sr.Tracks = append(sr.Tracks, fullTrackToSource(t))
		}
	}
	if result.Artists != nil {
		for _, a := range result.Artists.Artists {
			imgURL := ""
			if len(a.Images) > 0 {
				imgURL = a.Images[0].URL
			}
			sr.Artists = append(sr.Artists, source.SearchArtist{
				ID:       string(a.ID),
				Name:     a.Name,
				ImageURL: imgURL,
			})
		}
	}
	if result.Albums != nil {
		for _, a := range result.Albums.Albums {
			imgURL := ""
			if len(a.Images) > 0 {
				imgURL = a.Images[0].URL
			}
			artistName := ""
			if len(a.Artists) > 0 {
				artistName = a.Artists[0].Name
			}
			sr.Albums = append(sr.Albums, source.SearchAlbum{
				ID:       string(a.ID),
				Name:     a.Name,
				Artist:   artistName,
				ImageURL: imgURL,
			})
		}
	}

	return sr, nil
}

// GetArtist fetches artist details and top tracks from the Spotify API.
func (p *PlayerSource) GetArtist(ctx context.Context, artistID string) (*source.ArtistPage, error) {
	// Fetch artist details
	var artist struct {
		ID     string   `json:"id"`
		Name   string   `json:"name"`
		Genres []string `json:"genres"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
	}
	if err := p.apiGet(ctx, "/artists/"+artistID, &artist); err != nil {
		return nil, fmt.Errorf("get artist: %w", err)
	}

	// Fetch top tracks via the real endpoint (works with Extended Quota client ID)
	var topTracks struct {
		Tracks []apiTrack `json:"tracks"`
	}
	if err := p.apiGet(ctx, "/artists/"+artistID+"/top-tracks?market=from_token", &topTracks); err != nil {
		return nil, fmt.Errorf("get artist top tracks: %w", err)
	}

	tracks := make([]source.Track, 0, len(topTracks.Tracks))
	for _, t := range topTracks.Tracks {
		tracks = append(tracks, apiTrackToSource(t))
	}

	imageURL := ""
	if len(artist.Images) > 0 {
		imageURL = artist.Images[0].URL // largest image
	}

	// Fetch artist's albums (discography)
	var albumsResp struct {
		Items []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			ReleaseDate string `json:"release_date"`
			TotalTracks int    `json:"total_tracks"`
			AlbumType   string `json:"album_type"`
			Images      []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"items"`
	}
	var albums []source.ArtistAlbum
	if err := p.apiGet(ctx, "/artists/"+artistID+"/albums?include_groups=album,single&limit=20", &albumsResp); err == nil {
		for _, a := range albumsResp.Items {
			year := a.ReleaseDate
			if len(year) >= 4 {
				year = year[:4]
			}
			albumType := "Album"
			if a.AlbumType == "single" {
				albumType = "Single"
			}
			imgURL := ""
			if len(a.Images) > 0 {
				imgURL = a.Images[0].URL
			}
			albums = append(albums, source.ArtistAlbum{
				ID:       a.ID,
				Name:     a.Name,
				Year:     year,
				Type:     albumType,
				ImageURL: imgURL,
			})
		}
	}

	return &source.ArtistPage{
		Name:     artist.Name,
		ImageURL: imageURL,
		Genres:   artist.Genres,
		Tracks:   tracks,
		Albums:   albums,
	}, nil
}

// GetAlbum fetches album details and tracks from the Spotify API.
func (p *PlayerSource) GetAlbum(ctx context.Context, albumID string) (*source.AlbumPage, error) {
	var album struct {
		ID          string `json:"id"`
		URI         string `json:"uri"`
		Name        string `json:"name"`
		ReleaseDate string `json:"release_date"`
		Artists     []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"artists"`
		Images []struct {
			URL string `json:"url"`
		} `json:"images"`
		Tracks struct {
			Items []apiTrack `json:"items"`
		} `json:"tracks"`
	}
	if err := p.apiGet(ctx, "/albums/"+albumID, &album); err != nil {
		return nil, fmt.Errorf("get album: %w", err)
	}

	artistName := ""
	if len(album.Artists) > 0 {
		artistName = album.Artists[0].Name
	}

	imageURL := ""
	if len(album.Images) > 0 {
		imageURL = album.Images[0].URL // largest image
	}

	tracks := make([]source.Track, 0, len(album.Tracks.Items))
	for _, t := range album.Tracks.Items {
		st := apiTrackToSource(t)
		// Album tracks from the API don't include album info in each track,
		// so fill it in from the parent album.
		if st.Album == "" {
			st.Album = album.Name
		}
		if st.AlbumID == "" {
			st.AlbumID = album.ID
		}
		if st.ArtworkURL == "" {
			st.ArtworkURL = imageURL
		}
		tracks = append(tracks, st)
	}

	year := album.ReleaseDate
	if len(year) >= 4 {
		year = year[:4]
	}

	return &source.AlbumPage{
		ID:       album.ID,
		Name:     album.Name,
		Artist:   artistName,
		Year:     year,
		ImageURL: imageURL,
		Tracks:   tracks,
	}, nil
}
