package spotify

import (
	"errors"
	"testing"
	"time"

	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

// ---------------------------------------------------------------------------
// apiTrackToSource
// ---------------------------------------------------------------------------

func TestApiTrackToSource_FullFields(t *testing.T) {
	tr := apiTrack{
		ID:       "track123",
		URI:      "spotify:track:track123",
		Name:     "Test Song",
		Duration: 210000, // 3m30s
	}
	tr.Album.ID = "album456"
	tr.Album.Name = "Best Album"
	tr.Album.Images = []struct {
		URL string `json:"url"`
	}{{URL: "https://img.spotify.com/cover.jpg"}}
	tr.Artists = []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{{ID: "artist789", Name: "Cool Artist"}}

	got := apiTrackToSource(tr)

	assertEqual(t, "ID", got.ID, "track123")
	assertEqual(t, "URI", got.URI, "spotify:track:track123")
	assertEqual(t, "Name", got.Name, "Test Song")
	assertEqual(t, "Artist", got.Artist, "Cool Artist")
	assertEqual(t, "ArtistID", got.ArtistID, "artist789")
	assertEqual(t, "Album", got.Album, "Best Album")
	assertEqual(t, "AlbumID", got.AlbumID, "album456")
	assertEqual(t, "ArtworkURL", got.ArtworkURL, "https://img.spotify.com/cover.jpg")
	if got.Duration != 210*time.Second {
		t.Errorf("Duration = %v, want 3m30s", got.Duration)
	}
}

func TestApiTrackToSource_MissingArtist(t *testing.T) {
	tr := apiTrack{
		ID:   "noartist",
		Name: "Instrumental",
	}
	tr.Album.Name = "Some Album"

	got := apiTrackToSource(tr)

	assertEqual(t, "Artist", got.Artist, "")
	assertEqual(t, "ArtistID", got.ArtistID, "")
	assertEqual(t, "Name", got.Name, "Instrumental")
}

func TestApiTrackToSource_MissingAlbumImages(t *testing.T) {
	tr := apiTrack{
		ID:   "noimages",
		Name: "No Cover",
	}
	tr.Album.Name = "Imageless Album"
	tr.Artists = []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{{ID: "a1", Name: "Artist One"}}

	got := apiTrackToSource(tr)

	assertEqual(t, "ArtworkURL", got.ArtworkURL, "")
	assertEqual(t, "Album", got.Album, "Imageless Album")
	assertEqual(t, "Artist", got.Artist, "Artist One")
}

func TestApiTrackToSource_EmptyStruct(t *testing.T) {
	tr := apiTrack{}
	got := apiTrackToSource(tr)

	assertEqual(t, "ID", got.ID, "")
	assertEqual(t, "URI", got.URI, "")
	assertEqual(t, "Name", got.Name, "")
	assertEqual(t, "Artist", got.Artist, "")
	assertEqual(t, "ArtistID", got.ArtistID, "")
	assertEqual(t, "Album", got.Album, "")
	assertEqual(t, "AlbumID", got.AlbumID, "")
	assertEqual(t, "ArtworkURL", got.ArtworkURL, "")
	if got.Duration != 0 {
		t.Errorf("Duration = %v, want 0", got.Duration)
	}
}

// ---------------------------------------------------------------------------
// fullTrackToSource
// ---------------------------------------------------------------------------

func TestFullTrackToSource_FullFields(t *testing.T) {
	ft := spotifyapi.FullTrack{
		SimpleTrack: spotifyapi.SimpleTrack{
			ID:       "ft001",
			URI:      "spotify:track:ft001",
			Name:     "Full Track Song",
			Duration: 180000, // 3m
			Artists: []spotifyapi.SimpleArtist{
				{ID: "sa001", Name: "Full Artist"},
			},
		},
	}
	ft.Album = spotifyapi.SimpleAlbum{
		ID:   "fa001",
		Name: "Full Album",
		Images: []spotifyapi.Image{
			{URL: "https://img.spotify.com/full.jpg"},
		},
	}

	got := fullTrackToSource(ft)

	assertEqual(t, "ID", got.ID, "ft001")
	assertEqual(t, "URI", got.URI, "spotify:track:ft001")
	assertEqual(t, "Name", got.Name, "Full Track Song")
	assertEqual(t, "Artist", got.Artist, "Full Artist")
	assertEqual(t, "ArtistID", got.ArtistID, "sa001")
	assertEqual(t, "Album", got.Album, "Full Album")
	assertEqual(t, "AlbumID", got.AlbumID, "fa001")
	assertEqual(t, "ArtworkURL", got.ArtworkURL, "https://img.spotify.com/full.jpg")
	if got.Duration != 180*time.Second {
		t.Errorf("Duration = %v, want 3m0s", got.Duration)
	}
}

func TestFullTrackToSource_MissingArtist(t *testing.T) {
	ft := spotifyapi.FullTrack{
		SimpleTrack: spotifyapi.SimpleTrack{
			ID:   "ft002",
			Name: "No Artist Track",
		},
	}
	ft.Album = spotifyapi.SimpleAlbum{
		Name: "Some Album",
		Images: []spotifyapi.Image{
			{URL: "https://img.spotify.com/some.jpg"},
		},
	}

	got := fullTrackToSource(ft)

	assertEqual(t, "Artist", got.Artist, "")
	assertEqual(t, "ArtistID", got.ArtistID, "")
	assertEqual(t, "ArtworkURL", got.ArtworkURL, "https://img.spotify.com/some.jpg")
}

func TestFullTrackToSource_MissingAlbumImages(t *testing.T) {
	ft := spotifyapi.FullTrack{
		SimpleTrack: spotifyapi.SimpleTrack{
			ID:   "ft003",
			Name: "No Image Track",
			Artists: []spotifyapi.SimpleArtist{
				{ID: "sa003", Name: "Artist Three"},
			},
		},
	}
	ft.Album = spotifyapi.SimpleAlbum{
		Name: "Imageless",
	}

	got := fullTrackToSource(ft)

	assertEqual(t, "ArtworkURL", got.ArtworkURL, "")
	assertEqual(t, "Artist", got.Artist, "Artist Three")
}

// ---------------------------------------------------------------------------
// APIError
// ---------------------------------------------------------------------------

func TestAPIError_Error(t *testing.T) {
	e := &APIError{
		StatusCode: 404,
		Path:       "/me/tracks",
		Body:       `{"error":"not found"}`,
	}
	want := `spotify API /me/tracks: 404 {"error":"not found"}`
	if got := e.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestAPIError_HTTPStatus(t *testing.T) {
	tests := []struct {
		name string
		code int
	}{
		{"bad request", 400},
		{"unauthorized", 401},
		{"forbidden", 403},
		{"not found", 404},
		{"rate limited", 429},
		{"server error", 500},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := &APIError{StatusCode: tc.code, Path: "/test"}
			if got := e.HTTPStatus(); got != tc.code {
				t.Errorf("HTTPStatus() = %d, want %d", got, tc.code)
			}
		})
	}
}

// TestAPIError_TypeAssertion verifies that *APIError returned as a plain
// error can be unwrapped via errors.As, which is the pattern used by the
// retry logic in apiGetRetry to inspect status codes.
func TestAPIError_TypeAssertion(t *testing.T) {
	orig := &APIError{StatusCode: 429, Path: "/me/player", Body: "rate limited"}

	// Simulate returning as a plain error (like apiGetRetry does)
	var err error = orig

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("errors.As failed to extract *APIError")
	}
	if apiErr.HTTPStatus() != 429 {
		t.Errorf("HTTPStatus() = %d, want 429", apiErr.HTTPStatus())
	}
	if apiErr.Path != "/me/player" {
		t.Errorf("Path = %q, want %q", apiErr.Path, "/me/player")
	}
}

// TestAPIError_ErrorInterface confirms *APIError satisfies the error interface.
func TestAPIError_ErrorInterface(t *testing.T) {
	var _ error = (*APIError)(nil)
}

// ---------------------------------------------------------------------------
// apiTrackToSource — multiple artists (only first is used)
// ---------------------------------------------------------------------------

func TestApiTrackToSource_MultipleArtists(t *testing.T) {
	tr := apiTrack{
		ID:   "multi",
		Name: "Collab Song",
	}
	tr.Artists = []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}{
		{ID: "a1", Name: "Primary Artist"},
		{ID: "a2", Name: "Featured Artist"},
	}
	tr.Album.Name = "Joint Album"

	got := apiTrackToSource(tr)

	// Should only take the first artist
	assertEqual(t, "Artist", got.Artist, "Primary Artist")
	assertEqual(t, "ArtistID", got.ArtistID, "a1")
}

// TestApiTrackToSource_DurationConversion verifies millisecond-to-Duration math
// for a non-trivial value.
func TestApiTrackToSource_DurationConversion(t *testing.T) {
	tr := apiTrack{Duration: 123456}
	got := apiTrackToSource(tr)
	want := 123456 * time.Millisecond
	if got.Duration != want {
		t.Errorf("Duration = %v, want %v", got.Duration, want)
	}
}

// ---------------------------------------------------------------------------
// apiTrackToSource returns a complete source.Track
// ---------------------------------------------------------------------------

func TestApiTrackToSource_ReturnsSourceTrack(t *testing.T) {
	tr := apiTrack{ID: "x"}
	got := apiTrackToSource(tr)
	// Verify the return type is usable as source.Track (compile-time check
	// is sufficient, but we also confirm a zero-value field).
	var _ source.Track = got
	if got.Playing {
		t.Error("Playing should default to false")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func assertEqual(t *testing.T, field, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %q, want %q", field, got, want)
	}
}
