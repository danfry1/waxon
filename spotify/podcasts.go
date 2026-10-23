package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/danfry1/waxon/source"
)

// isURI reports whether a track reference is a full Spotify URI rather than
// a bare track ID. Episodes are referred to by URI ("spotify:episode:…")
// throughout waxon, so library and queue calls accept either.
func isURI(ref string) bool { return strings.HasPrefix(ref, "spotify:") }

// itemURI turns a track reference into a URI: bare IDs are tracks.
func itemURI(ref string) string {
	if isURI(ref) {
		return ref
	}
	return "spotify:track:" + ref
}

type apiShowRef = struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Publisher string     `json:"publisher"`
	Images    []apiImage `json:"images"`
}

type apiEpisode struct {
	ID          string      `json:"id"`
	URI         string      `json:"uri"`
	Name        string      `json:"name"`
	Duration    int         `json:"duration_ms"`
	ReleaseDate string      `json:"release_date"`
	Images      []apiImage  `json:"images"`
	Show        *apiShowRef `json:"show"`
}

// apiEpisodeToSource maps an episode onto a Track: the show stands in for the
// artist and its publisher for the album. ID is the episode URI, so it can't
// be mistaken for a track ID.
func apiEpisodeToSource(e apiEpisode) source.Track {
	t := source.Track{
		ID:         e.URI,
		URI:        e.URI,
		Name:       e.Name,
		Duration:   time.Duration(e.Duration) * time.Millisecond,
		ArtworkURL: pickImageURL(e.Images, trackArtMinPx),
		IsEpisode:  true,
	}
	if e.Show != nil {
		t.Artist, t.Album, t.ShowID = e.Show.Name, e.Show.Publisher, e.Show.ID
		if t.ArtworkURL == "" {
			t.ArtworkURL = pickImageURL(e.Show.Images, trackArtMinPx)
		}
	}
	if t.URI == "" && e.ID != "" {
		t.ID, t.URI = "spotify:episode:"+e.ID, "spotify:episode:"+e.ID
	}
	return t
}

// playingItem decodes the player's item as a track or an episode.
func playingItem(kind string, raw json.RawMessage) (source.Track, bool) {
	switch kind {
	case "", "track":
		var t apiTrack
		if json.Unmarshal(raw, &t) != nil || t.ID == "" {
			return source.Track{}, false
		}
		return apiTrackToSource(t), true
	case "episode":
		var e apiEpisode
		if json.Unmarshal(raw, &e) != nil || (e.ID == "" && e.URI == "") {
			return source.Track{}, false
		}
		return apiEpisodeToSource(e), true
	}
	return source.Track{}, false // ads, "unknown"
}

// SavedShows lists the podcasts the user follows.
func (p *PlayerSource) SavedShows(ctx context.Context) ([]source.Show, error) {
	var shows []source.Show
	for offset := 0; offset < maxLibraryItems; offset += 50 {
		var page struct {
			Items []struct {
				Show struct {
					ID        string     `json:"id"`
					URI       string     `json:"uri"`
					Name      string     `json:"name"`
					Publisher string     `json:"publisher"`
					Images    []apiImage `json:"images"`
				} `json:"show"`
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := p.apiGet(ctx, fmt.Sprintf("/me/shows?limit=50&offset=%d", offset), &page); err != nil {
			return shows, wrapScopeError(err)
		}
		for _, it := range page.Items {
			s := it.Show
			shows = append(shows, source.Show{ID: s.ID, URI: s.URI, Name: s.Name, Publisher: s.Publisher, ImageURL: pickImageURL(s.Images, thumbArtMinPx)})
		}
		if page.Next == "" || len(page.Items) == 0 {
			break
		}
	}
	return shows, nil
}

// ShowEpisodesPage lists a show's episodes, newest first.
func (p *PlayerSource) ShowEpisodesPage(ctx context.Context, showID string, offset, limit int) ([]source.Track, int, error) {
	var page struct {
		Items []*apiEpisode `json:"items"` // unavailable episodes come back as null
		Total int           `json:"total"`
	}
	path := fmt.Sprintf("/shows/%s/episodes?limit=%d&offset=%d&market=from_token", showID, min(limit, 50), offset)
	if err := p.apiGet(ctx, path, &page); err != nil {
		return nil, 0, err
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, e := range page.Items {
		if e == nil {
			// Keep the slot (as a blank, unselectable row) so the caller's
			// next offset, which counts rows, stays in step with Spotify's.
			tracks = append(tracks, source.Track{IsSeparator: true})
			continue
		}
		t := apiEpisodeToSource(*e)
		t.ShowID = showID
		t.Album = e.ReleaseDate // the show is the page; the date tells episodes apart
		tracks = append(tracks, t)
	}
	return tracks, page.Total, nil
}
