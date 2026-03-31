package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	spotifyapi "github.com/zmb3/spotify/v2"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// rewriteTransport redirects all HTTP requests to the test server by
// rewriting the scheme and host before forwarding to the default transport.
type rewriteTransport struct {
	base    http.RoundTripper
	testURL string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = strings.TrimPrefix(t.testURL, "http://")
	return t.base.RoundTrip(req)
}

// newTestPlayerSource creates a PlayerSource whose httpClient routes all
// requests through the given test server URL.
func newTestPlayerSource(serverURL string) *PlayerSource {
	return &PlayerSource{
		httpClient: &http.Client{
			Transport: &rewriteTransport{
				base:    http.DefaultTransport,
				testURL: serverURL,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// apiGet / apiGetRetry
// ---------------------------------------------------------------------------

func TestApiGetSuccess(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload{Name: "hello", Count: 42})
	}))
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	var got payload
	if err := ps.apiGet(context.Background(), "/test", &got); err != nil {
		t.Fatalf("apiGet returned error: %v", err)
	}
	if got.Name != "hello" {
		t.Errorf("Name = %q, want %q", got.Name, "hello")
	}
	if got.Count != 42 {
		t.Errorf("Count = %d, want %d", got.Count, 42)
	}
}

func TestApiGetNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	var v struct{}
	err := ps.apiGet(context.Background(), "/missing", &v)
	if err == nil {
		t.Fatal("expected error for 404 response")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", apiErr.StatusCode)
	}
	if apiErr.Path != "/missing" {
		t.Errorf("Path = %q, want %q", apiErr.Path, "/missing")
	}
}

func TestApiGetRateLimit(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.Header().Set("Retry-After", "0") // use 0 to not actually wait; falls to default 1s
			w.WriteHeader(429)
			fmt.Fprint(w, `{"error":"rate limited"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	var got struct {
		OK bool `json:"ok"`
	}
	// Note: this will sleep for 1 second on the retry (default when Retry-After
	// is 0/invalid). We accept the 1s penalty for correctness.
	err := ps.apiGet(context.Background(), "/rate", &got)
	if err != nil {
		t.Fatalf("apiGet returned error: %v", err)
	}
	if !got.OK {
		t.Error("expected OK=true after retry")
	}
	if c := atomic.LoadInt32(&calls); c != 2 {
		t.Errorf("expected 2 calls, got %d", c)
	}
}

func TestApiGetRateLimitExhausted(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(429)
		fmt.Fprint(w, `rate limited`)
	}))
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	var v struct{}
	err := ps.apiGet(context.Background(), "/exhaust", &v)
	if err == nil {
		t.Fatal("expected error after exhausting retries")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != 429 {
		t.Errorf("StatusCode = %d, want 429", apiErr.StatusCode)
	}

	// Should have been called maxRetries+1 times (initial + retries, then
	// the final attempt falls through to the non-200 handler)
	expectedCalls := int32(maxRetries + 1)
	if c := atomic.LoadInt32(&calls); c != expectedCalls {
		t.Errorf("expected %d calls, got %d", expectedCalls, c)
	}
}

func TestApiGetNetworkError(t *testing.T) {
	// Use a client that points at nothing (closed server).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	srv.Close() // immediately close to simulate network failure

	ps := newTestPlayerSource(srv.URL)

	var v struct{}
	err := ps.apiGet(context.Background(), "/unreachable", &v)
	if err == nil {
		t.Fatal("expected error for network failure")
	}
	// Should NOT be an APIError — it's a transport-level error.
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("expected transport error, got APIError: %v", apiErr)
	}
}

func TestApiGetInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprint(w, `{invalid json!!!`)
	}))
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	var v struct {
		Name string `json:"name"`
	}
	err := ps.apiGet(context.Background(), "/badjson", &v)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	// Should NOT be an APIError — it's a decode error on a 200 response.
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Errorf("expected decode error, got APIError: %v", apiErr)
	}
}

// ---------------------------------------------------------------------------
// Playlists
// ---------------------------------------------------------------------------

func TestPlaylistsParsing(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/me/tracks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total": 137}`)
	})

	mux.HandleFunc("/v1/me/playlists", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"items": [
				{
					"id": "pl001",
					"uri": "spotify:playlist:pl001",
					"name": "My Playlist",
					"images": [{"url": "https://img.spotify.com/pl.jpg"}],
					"items": {"total": 25},
					"tracks": {"total": 0}
				},
				{
					"id": "pl002",
					"uri": "spotify:playlist:pl002",
					"name": "Another Playlist",
					"images": [],
					"items": {"total": 0},
					"tracks": {"total": 10}
				}
			]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	playlists, err := ps.Playlists(context.Background())
	if err != nil {
		t.Fatalf("Playlists returned error: %v", err)
	}

	if len(playlists) != 3 {
		t.Fatalf("expected 3 playlists (liked + 2), got %d", len(playlists))
	}

	// First entry is always Liked Songs
	liked := playlists[0]
	if liked.ID != LikedPlaylistID {
		t.Errorf("first playlist ID = %q, want %q", liked.ID, LikedPlaylistID)
	}
	if liked.Name != "♥ Liked Songs" {
		t.Errorf("first playlist Name = %q, want %q", liked.Name, "♥ Liked Songs")
	}
	if liked.TrackCount != 137 {
		t.Errorf("liked TrackCount = %d, want 137", liked.TrackCount)
	}

	// Second entry: My Playlist (uses items.total)
	pl1 := playlists[1]
	if pl1.ID != "pl001" {
		t.Errorf("pl1 ID = %q, want %q", pl1.ID, "pl001")
	}
	if pl1.Name != "My Playlist" {
		t.Errorf("pl1 Name = %q, want %q", pl1.Name, "My Playlist")
	}
	if pl1.ImageURL != "https://img.spotify.com/pl.jpg" {
		t.Errorf("pl1 ImageURL = %q, want %q", pl1.ImageURL, "https://img.spotify.com/pl.jpg")
	}
	if pl1.TrackCount != 25 {
		t.Errorf("pl1 TrackCount = %d, want 25", pl1.TrackCount)
	}

	// Third entry: Another Playlist (falls back to tracks.total)
	pl2 := playlists[2]
	if pl2.ID != "pl002" {
		t.Errorf("pl2 ID = %q, want %q", pl2.ID, "pl002")
	}
	if pl2.ImageURL != "" {
		t.Errorf("pl2 ImageURL = %q, want empty string", pl2.ImageURL)
	}
	if pl2.TrackCount != 10 {
		t.Errorf("pl2 TrackCount = %d, want 10 (fallback to tracks.total)", pl2.TrackCount)
	}
}

// ---------------------------------------------------------------------------
// PlaylistTracksPage
// ---------------------------------------------------------------------------

func TestPlaylistTracksPageFirstPage(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/playlists/abc123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"name": "Test Playlist",
			"items": {
				"total": 2,
				"items": [
					{
						"item": {
							"id": "t1",
							"uri": "spotify:track:t1",
							"name": "Track One",
							"duration_ms": 200000,
							"album": {
								"id": "alb1",
								"name": "Album One",
								"images": [{"url": "https://img/a1.jpg"}]
							},
							"artists": [{"id": "ar1", "name": "Artist One"}]
						}
					},
					{
						"item": {
							"id": "t2",
							"uri": "spotify:track:t2",
							"name": "Track Two",
							"duration_ms": 180000,
							"album": {
								"id": "alb2",
								"name": "Album Two",
								"images": []
							},
							"artists": [{"id": "ar2", "name": "Artist Two"}]
						}
					}
				]
			}
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	tracks, total, err := ps.PlaylistTracksPage(context.Background(), "abc123", 0, 50)
	if err != nil {
		t.Fatalf("PlaylistTracksPage returned error: %v", err)
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}

	if tracks[0].ID != "t1" {
		t.Errorf("tracks[0].ID = %q, want %q", tracks[0].ID, "t1")
	}
	if tracks[0].Name != "Track One" {
		t.Errorf("tracks[0].Name = %q, want %q", tracks[0].Name, "Track One")
	}
	if tracks[0].Artist != "Artist One" {
		t.Errorf("tracks[0].Artist = %q, want %q", tracks[0].Artist, "Artist One")
	}
	if tracks[0].Album != "Album One" {
		t.Errorf("tracks[0].Album = %q, want %q", tracks[0].Album, "Album One")
	}
	if tracks[0].ArtworkURL != "https://img/a1.jpg" {
		t.Errorf("tracks[0].ArtworkURL = %q, want %q", tracks[0].ArtworkURL, "https://img/a1.jpg")
	}
	if tracks[0].Duration != 200*time.Second {
		t.Errorf("tracks[0].Duration = %v, want 200s", tracks[0].Duration)
	}

	if tracks[1].ID != "t2" {
		t.Errorf("tracks[1].ID = %q, want %q", tracks[1].ID, "t2")
	}
	if tracks[1].ArtworkURL != "" {
		t.Errorf("tracks[1].ArtworkURL = %q, want empty (no images)", tracks[1].ArtworkURL)
	}
}

func TestPlaylistTracksPageLiked(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/me/tracks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"total": 1,
			"items": [
				{
					"track": {
						"id": "liked1",
						"uri": "spotify:track:liked1",
						"name": "Liked Track",
						"duration_ms": 240000,
						"album": {"id": "la1", "name": "Liked Album", "images": [{"url": "https://img/liked.jpg"}]},
						"artists": [{"id": "lar1", "name": "Liked Artist"}]
					}
				}
			]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	tracks, total, err := ps.PlaylistTracksPage(context.Background(), LikedPlaylistID, 0, 50)
	if err != nil {
		t.Fatalf("PlaylistTracksPage(liked) returned error: %v", err)
	}
	if total != 1 {
		t.Errorf("total = %d, want 1", total)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}
	if tracks[0].ID != "liked1" {
		t.Errorf("tracks[0].ID = %q, want %q", tracks[0].ID, "liked1")
	}
	if tracks[0].Name != "Liked Track" {
		t.Errorf("tracks[0].Name = %q, want %q", tracks[0].Name, "Liked Track")
	}
}

func TestPlaylistTracksPageOffset(t *testing.T) {
	var requestedPath string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/playlists/", func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path + "?" + r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"total": 100,
			"items": [
				{
					"item": {
						"id": "off1",
						"uri": "spotify:track:off1",
						"name": "Offset Track",
						"duration_ms": 120000,
						"album": {"id": "oa1", "name": "Offset Album", "images": []},
						"artists": [{"id": "oar1", "name": "Offset Artist"}]
					}
				}
			]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	tracks, total, err := ps.PlaylistTracksPage(context.Background(), "pl999", 50, 25)
	if err != nil {
		t.Fatalf("PlaylistTracksPage(offset) returned error: %v", err)
	}
	if total != 100 {
		t.Errorf("total = %d, want 100", total)
	}
	if len(tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(tracks))
	}

	// Verify the URL included the offset and limit
	wantPath := "/v1/playlists/pl999/tracks?offset=50&limit=25"
	if requestedPath != wantPath {
		t.Errorf("requested path = %q, want %q", requestedPath, wantPath)
	}
}

// ---------------------------------------------------------------------------
// LikedTracksPage
// ---------------------------------------------------------------------------

func TestLikedTracksPageLimitCap(t *testing.T) {
	var requestedURL string

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/me/tracks", func(w http.ResponseWriter, r *http.Request) {
		requestedURL = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total": 500, "items": []}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	_, total, err := ps.LikedTracksPage(context.Background(), 0, 200)
	if err != nil {
		t.Fatalf("LikedTracksPage returned error: %v", err)
	}
	if total != 500 {
		t.Errorf("total = %d, want 500", total)
	}

	// The limit should have been capped to 50
	if !strings.Contains(requestedURL, "limit=50") {
		t.Errorf("expected limit=50 in URL, got %q", requestedURL)
	}
	if strings.Contains(requestedURL, "limit=200") {
		t.Errorf("limit should have been capped to 50, but URL contains limit=200: %q", requestedURL)
	}
}

// ---------------------------------------------------------------------------
// GetArtist
// ---------------------------------------------------------------------------

func TestGetArtistParsing(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/artists/art42/top-tracks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tracks": [
				{
					"id": "tt1",
					"uri": "spotify:track:tt1",
					"name": "Top Hit",
					"duration_ms": 195000,
					"album": {"id": "tta1", "name": "Hit Album", "images": [{"url": "https://img/hit.jpg"}]},
					"artists": [{"id": "art42", "name": "Test Artist"}]
				}
			]
		}`)
	})

	mux.HandleFunc("/v1/artists/art42/albums", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"items": [
				{
					"id": "disc1",
					"name": "Debut Album",
					"release_date": "2020-03-15",
					"total_tracks": 12,
					"album_type": "album",
					"images": [{"url": "https://img/debut.jpg"}]
				},
				{
					"id": "disc2",
					"name": "Hot Single",
					"release_date": "2021-07",
					"total_tracks": 1,
					"album_type": "single",
					"images": []
				}
			]
		}`)
	})

	mux.HandleFunc("/v1/artists/art42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "art42",
			"name": "Test Artist",
			"genres": ["indie", "rock"],
			"images": [{"url": "https://img/artist.jpg"}]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	page, err := ps.GetArtist(context.Background(), "art42")
	if err != nil {
		t.Fatalf("GetArtist returned error: %v", err)
	}

	if page.Name != "Test Artist" {
		t.Errorf("Name = %q, want %q", page.Name, "Test Artist")
	}
	if page.ImageURL != "https://img/artist.jpg" {
		t.Errorf("ImageURL = %q, want %q", page.ImageURL, "https://img/artist.jpg")
	}
	if len(page.Genres) != 2 || page.Genres[0] != "indie" || page.Genres[1] != "rock" {
		t.Errorf("Genres = %v, want [indie rock]", page.Genres)
	}

	// Top tracks
	if len(page.Tracks) != 1 {
		t.Fatalf("expected 1 top track, got %d", len(page.Tracks))
	}
	if page.Tracks[0].ID != "tt1" {
		t.Errorf("Tracks[0].ID = %q, want %q", page.Tracks[0].ID, "tt1")
	}
	if page.Tracks[0].Name != "Top Hit" {
		t.Errorf("Tracks[0].Name = %q, want %q", page.Tracks[0].Name, "Top Hit")
	}
	if page.Tracks[0].Duration != 195*time.Second {
		t.Errorf("Tracks[0].Duration = %v, want 195s", page.Tracks[0].Duration)
	}

	// Albums (discography)
	if len(page.Albums) != 2 {
		t.Fatalf("expected 2 albums, got %d", len(page.Albums))
	}
	if page.Albums[0].ID != "disc1" {
		t.Errorf("Albums[0].ID = %q, want %q", page.Albums[0].ID, "disc1")
	}
	if page.Albums[0].Name != "Debut Album" {
		t.Errorf("Albums[0].Name = %q, want %q", page.Albums[0].Name, "Debut Album")
	}
	if page.Albums[0].Year != "2020" {
		t.Errorf("Albums[0].Year = %q, want %q", page.Albums[0].Year, "2020")
	}
	if page.Albums[0].Type != "Album" {
		t.Errorf("Albums[0].Type = %q, want %q", page.Albums[0].Type, "Album")
	}
	if page.Albums[0].ImageURL != "https://img/debut.jpg" {
		t.Errorf("Albums[0].ImageURL = %q, want %q", page.Albums[0].ImageURL, "https://img/debut.jpg")
	}

	if page.Albums[1].Name != "Hot Single" {
		t.Errorf("Albums[1].Name = %q, want %q", page.Albums[1].Name, "Hot Single")
	}
	if page.Albums[1].Type != "Single" {
		t.Errorf("Albums[1].Type = %q, want %q", page.Albums[1].Type, "Single")
	}
	if page.Albums[1].Year != "2021" {
		t.Errorf("Albums[1].Year = %q, want %q", page.Albums[1].Year, "2021")
	}
	if page.Albums[1].ImageURL != "" {
		t.Errorf("Albums[1].ImageURL = %q, want empty", page.Albums[1].ImageURL)
	}
}

// ---------------------------------------------------------------------------
// GetAlbum
// ---------------------------------------------------------------------------

func TestGetAlbumParsing(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/albums/alb77", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "alb77",
			"uri": "spotify:album:alb77",
			"name": "Great Album",
			"release_date": "2019-11-22",
			"artists": [{"id": "albArt1", "name": "Album Artist"}],
			"images": [{"url": "https://img/album.jpg"}],
			"tracks": {
				"items": [
					{
						"id": "at1",
						"uri": "spotify:track:at1",
						"name": "Album Track 1",
						"duration_ms": 210000,
						"album": {"id": "", "name": "", "images": []},
						"artists": [{"id": "albArt1", "name": "Album Artist"}]
					},
					{
						"id": "at2",
						"uri": "spotify:track:at2",
						"name": "Album Track 2",
						"duration_ms": 185000,
						"album": {"id": "", "name": "", "images": []},
						"artists": []
					}
				]
			}
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	page, err := ps.GetAlbum(context.Background(), "alb77")
	if err != nil {
		t.Fatalf("GetAlbum returned error: %v", err)
	}

	if page.ID != "alb77" {
		t.Errorf("ID = %q, want %q", page.ID, "alb77")
	}
	if page.Name != "Great Album" {
		t.Errorf("Name = %q, want %q", page.Name, "Great Album")
	}
	if page.Artist != "Album Artist" {
		t.Errorf("Artist = %q, want %q", page.Artist, "Album Artist")
	}
	if page.Year != "2019" {
		t.Errorf("Year = %q, want %q", page.Year, "2019")
	}
	if page.ImageURL != "https://img/album.jpg" {
		t.Errorf("ImageURL = %q, want %q", page.ImageURL, "https://img/album.jpg")
	}

	if len(page.Tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(page.Tracks))
	}

	// Album tracks should have album info backfilled
	tr1 := page.Tracks[0]
	if tr1.ID != "at1" {
		t.Errorf("Tracks[0].ID = %q, want %q", tr1.ID, "at1")
	}
	if tr1.Album != "Great Album" {
		t.Errorf("Tracks[0].Album = %q, want %q (backfilled)", tr1.Album, "Great Album")
	}
	if tr1.AlbumID != "alb77" {
		t.Errorf("Tracks[0].AlbumID = %q, want %q (backfilled)", tr1.AlbumID, "alb77")
	}
	if tr1.ArtworkURL != "https://img/album.jpg" {
		t.Errorf("Tracks[0].ArtworkURL = %q, want %q (backfilled)", tr1.ArtworkURL, "https://img/album.jpg")
	}
	if tr1.Artist != "Album Artist" {
		t.Errorf("Tracks[0].Artist = %q, want %q", tr1.Artist, "Album Artist")
	}

	// Second track has no artists
	tr2 := page.Tracks[1]
	if tr2.ID != "at2" {
		t.Errorf("Tracks[1].ID = %q, want %q", tr2.ID, "at2")
	}
	if tr2.Artist != "" {
		t.Errorf("Tracks[1].Artist = %q, want empty", tr2.Artist)
	}
	if tr2.Album != "Great Album" {
		t.Errorf("Tracks[1].Album = %q, want %q (backfilled)", tr2.Album, "Great Album")
	}
}

// ---------------------------------------------------------------------------
// RecentlyPlayed
// ---------------------------------------------------------------------------

func TestRecentlyPlayedParsing(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/me/player/recently-played", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"items": [
				{
					"track": {
						"id": "rp1",
						"uri": "spotify:track:rp1",
						"name": "Recently Played 1",
						"duration_ms": 300000,
						"album": {"id": "rpa1", "name": "Recent Album", "images": [{"url": "https://img/recent.jpg"}]},
						"artists": [{"id": "rpar1", "name": "Recent Artist"}]
					}
				},
				{
					"track": {
						"id": "rp2",
						"uri": "spotify:track:rp2",
						"name": "Recently Played 2",
						"duration_ms": 250000,
						"album": {"id": "rpa2", "name": "Another Recent", "images": []},
						"artists": [{"id": "rpar2", "name": "Another Recent Artist"}]
					}
				}
			]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	tracks, err := ps.RecentlyPlayed(context.Background())
	if err != nil {
		t.Fatalf("RecentlyPlayed returned error: %v", err)
	}

	if len(tracks) != 2 {
		t.Fatalf("expected 2 tracks, got %d", len(tracks))
	}

	if tracks[0].ID != "rp1" {
		t.Errorf("tracks[0].ID = %q, want %q", tracks[0].ID, "rp1")
	}
	if tracks[0].Name != "Recently Played 1" {
		t.Errorf("tracks[0].Name = %q, want %q", tracks[0].Name, "Recently Played 1")
	}
	if tracks[0].Artist != "Recent Artist" {
		t.Errorf("tracks[0].Artist = %q, want %q", tracks[0].Artist, "Recent Artist")
	}
	if tracks[0].ArtworkURL != "https://img/recent.jpg" {
		t.Errorf("tracks[0].ArtworkURL = %q, want %q", tracks[0].ArtworkURL, "https://img/recent.jpg")
	}
	if tracks[0].Duration != 300*time.Second {
		t.Errorf("tracks[0].Duration = %v, want 300s", tracks[0].Duration)
	}

	if tracks[1].ID != "rp2" {
		t.Errorf("tracks[1].ID = %q, want %q", tracks[1].ID, "rp2")
	}
	if tracks[1].ArtworkURL != "" {
		t.Errorf("tracks[1].ArtworkURL = %q, want empty", tracks[1].ArtworkURL)
	}
}

// ---------------------------------------------------------------------------
// Helper: PlayerSource with a real zmb3 client pointed at a test server
// ---------------------------------------------------------------------------

// newTestPlayerSourceWithClient creates a PlayerSource whose zmb3 client
// (and httpClient) routes all requests through the given test server URL.
func newTestPlayerSourceWithClient(serverURL string) *PlayerSource {
	httpClient := &http.Client{
		Transport: &rewriteTransport{
			base:    http.DefaultTransport,
			testURL: serverURL,
		},
	}
	client := spotifyapi.New(httpClient)
	return &PlayerSource{
		client:     client,
		httpClient: httpClient,
		features:   NewFeatureCache(client),
	}
}

// ---------------------------------------------------------------------------
// Search (via zmb3 client)
// ---------------------------------------------------------------------------

func TestSearchParsing(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tracks": {
				"items": [
					{
						"id": "t1",
						"name": "Song",
						"uri": "spotify:track:t1",
						"duration_ms": 200000,
						"artists": [{"id": "a1", "name": "Artist"}],
						"album": {"id": "al1", "name": "Album", "images": [{"url": "http://img"}]}
					}
				]
			},
			"artists": {
				"items": [
					{
						"id": "a1",
						"name": "Artist",
						"images": [{"url": "http://art"}]
					}
				]
			},
			"albums": {
				"items": [
					{
						"id": "al1",
						"name": "Album",
						"artists": [{"id": "a1", "name": "ArtistName"}],
						"images": [{"url": "http://alb"}]
					}
				]
			}
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSourceWithClient(srv.URL)

	results, err := ps.Search(context.Background(), "test query")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}

	// Tracks
	if len(results.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(results.Tracks))
	}
	if results.Tracks[0].ID != "t1" {
		t.Errorf("track ID = %q, want %q", results.Tracks[0].ID, "t1")
	}
	if results.Tracks[0].Name != "Song" {
		t.Errorf("track Name = %q, want %q", results.Tracks[0].Name, "Song")
	}
	if results.Tracks[0].Artist != "Artist" {
		t.Errorf("track Artist = %q, want %q", results.Tracks[0].Artist, "Artist")
	}
	if results.Tracks[0].Album != "Album" {
		t.Errorf("track Album = %q, want %q", results.Tracks[0].Album, "Album")
	}
	if results.Tracks[0].ArtworkURL != "http://img" {
		t.Errorf("track ArtworkURL = %q, want %q", results.Tracks[0].ArtworkURL, "http://img")
	}
	if results.Tracks[0].Duration != 200*time.Second {
		t.Errorf("track Duration = %v, want 200s", results.Tracks[0].Duration)
	}

	// Artists
	if len(results.Artists) != 1 {
		t.Fatalf("expected 1 artist, got %d", len(results.Artists))
	}
	if results.Artists[0].ID != "a1" {
		t.Errorf("artist ID = %q, want %q", results.Artists[0].ID, "a1")
	}
	if results.Artists[0].Name != "Artist" {
		t.Errorf("artist Name = %q, want %q", results.Artists[0].Name, "Artist")
	}
	if results.Artists[0].ImageURL != "http://art" {
		t.Errorf("artist ImageURL = %q, want %q", results.Artists[0].ImageURL, "http://art")
	}

	// Albums
	if len(results.Albums) != 1 {
		t.Fatalf("expected 1 album, got %d", len(results.Albums))
	}
	if results.Albums[0].ID != "al1" {
		t.Errorf("album ID = %q, want %q", results.Albums[0].ID, "al1")
	}
	if results.Albums[0].Name != "Album" {
		t.Errorf("album Name = %q, want %q", results.Albums[0].Name, "Album")
	}
	if results.Albums[0].Artist != "ArtistName" {
		t.Errorf("album Artist = %q, want %q", results.Albums[0].Artist, "ArtistName")
	}
	if results.Albums[0].ImageURL != "http://alb" {
		t.Errorf("album ImageURL = %q, want %q", results.Albums[0].ImageURL, "http://alb")
	}
}

func TestSearchEmpty(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tracks": {"items": []},
			"artists": {"items": []},
			"albums": {"items": []}
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSourceWithClient(srv.URL)

	results, err := ps.Search(context.Background(), "nonexistent")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results.Tracks) != 0 {
		t.Errorf("expected 0 tracks, got %d", len(results.Tracks))
	}
	if len(results.Artists) != 0 {
		t.Errorf("expected 0 artists, got %d", len(results.Artists))
	}
	if len(results.Albums) != 0 {
		t.Errorf("expected 0 albums, got %d", len(results.Albums))
	}
}

// ---------------------------------------------------------------------------
// Playlists pagination
// ---------------------------------------------------------------------------

func TestPlaylistsPagination(t *testing.T) {
	var callCount int32

	mux := http.NewServeMux()

	mux.HandleFunc("/v1/me/tracks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total": 10}`)
	})

	mux.HandleFunc("/v1/me/playlists", func(w http.ResponseWriter, r *http.Request) {
		page := atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")

		if page == 1 {
			// First page: return exactly 50 items (signals more pages)
			items := make([]map[string]interface{}, 50)
			for i := 0; i < 50; i++ {
				items[i] = map[string]interface{}{
					"id":     fmt.Sprintf("pl-%03d", i),
					"uri":    fmt.Sprintf("spotify:playlist:pl-%03d", i),
					"name":   fmt.Sprintf("Playlist %d", i),
					"images": []interface{}{},
					"items":  map[string]int{"total": i + 1},
					"tracks": map[string]int{"total": 0},
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
		} else {
			// Second page: return fewer than 50 (signals last page)
			items := make([]map[string]interface{}, 3)
			for i := 0; i < 3; i++ {
				items[i] = map[string]interface{}{
					"id":     fmt.Sprintf("pl-extra-%d", i),
					"uri":    fmt.Sprintf("spotify:playlist:pl-extra-%d", i),
					"name":   fmt.Sprintf("Extra Playlist %d", i),
					"images": []interface{}{},
					"items":  map[string]int{"total": 5},
					"tracks": map[string]int{"total": 0},
				}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"items": items})
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	playlists, err := ps.Playlists(context.Background())
	if err != nil {
		t.Fatalf("Playlists returned error: %v", err)
	}

	// 1 (liked) + 50 (first page) + 3 (second page) = 54
	if len(playlists) != 54 {
		t.Errorf("expected 54 playlists, got %d", len(playlists))
	}

	// Should have made exactly 2 API calls to /me/playlists
	if c := atomic.LoadInt32(&callCount); c != 2 {
		t.Errorf("expected 2 playlist API calls, got %d", c)
	}

	// Verify first is liked
	if playlists[0].ID != LikedPlaylistID {
		t.Errorf("first playlist should be liked, got %q", playlists[0].ID)
	}

	// Verify last item is from second page
	last := playlists[len(playlists)-1]
	if last.ID != "pl-extra-2" {
		t.Errorf("last playlist ID = %q, want %q", last.ID, "pl-extra-2")
	}
}

// ---------------------------------------------------------------------------
// GetArtist album fallback
// ---------------------------------------------------------------------------

func TestGetArtistAlbumFallback(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/artists/art99", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "art99",
			"name": "Fallback Artist",
			"genres": ["pop"],
			"images": [{"url": "http://artist.jpg"}]
		}`)
	})

	mux.HandleFunc("/v1/artists/art99/top-tracks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"tracks": [
				{
					"id": "ft1",
					"uri": "spotify:track:ft1",
					"name": "Fallback Hit",
					"duration_ms": 180000,
					"album": {"id": "fa1", "name": "FallAlbum", "images": [{"url": "http://fa.jpg"}]},
					"artists": [{"id": "art99", "name": "Fallback Artist"}]
				}
			]
		}`)
	})

	mux.HandleFunc("/v1/artists/art99/albums", func(w http.ResponseWriter, r *http.Request) {
		// Return error for albums endpoint
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"internal server error"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	page, err := ps.GetArtist(context.Background(), "art99")
	if err != nil {
		t.Fatalf("GetArtist returned error: %v", err)
	}

	// Artist details should be present
	if page.Name != "Fallback Artist" {
		t.Errorf("Name = %q, want %q", page.Name, "Fallback Artist")
	}

	// Top tracks should still be present
	if len(page.Tracks) != 1 {
		t.Fatalf("expected 1 track, got %d", len(page.Tracks))
	}
	if page.Tracks[0].ID != "ft1" {
		t.Errorf("track ID = %q, want %q", page.Tracks[0].ID, "ft1")
	}

	// Albums should be empty (fallback on error)
	if len(page.Albums) != 0 {
		t.Errorf("expected 0 albums on error, got %d", len(page.Albums))
	}
}

// ---------------------------------------------------------------------------
// FeatureCache.Get with real API (via zmb3 client + httptest)
// ---------------------------------------------------------------------------

func TestFeatureCacheGetViaMock(t *testing.T) {
	var callCount int32

	mux := http.NewServeMux()
	mux.HandleFunc("/v1/audio-features/", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"audio_features": [
				{
					"energy": 0.85,
					"valence": 0.72,
					"danceability": 0.65,
					"tempo": 128.0,
					"acousticness": 0.15,
					"id": "track123"
				}
			]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSourceWithClient(srv.URL)

	// First call: cache miss, calls API
	af, err := ps.features.Get(context.Background(), "track123")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if af == nil {
		t.Fatal("expected non-nil AudioFeatures")
	}
	// The zmb3 library uses float32 internally, so we compare against
	// float64(float32(x)) to avoid precision mismatch.
	if af.Energy != float64(float32(0.85)) {
		t.Errorf("Energy = %f, want ~0.85", af.Energy)
	}
	if af.Valence != float64(float32(0.72)) {
		t.Errorf("Valence = %f, want ~0.72", af.Valence)
	}
	if af.Danceability != float64(float32(0.65)) {
		t.Errorf("Danceability = %f, want ~0.65", af.Danceability)
	}
	if af.Tempo != float64(float32(128.0)) {
		t.Errorf("Tempo = %f, want ~128.0", af.Tempo)
	}
	if af.Acousticness != float64(float32(0.15)) {
		t.Errorf("Acousticness = %f, want ~0.15", af.Acousticness)
	}

	if c := atomic.LoadInt32(&callCount); c != 1 {
		t.Errorf("expected 1 API call, got %d", c)
	}

	// Second call: cache hit, should NOT call API again
	af2, err := ps.features.Get(context.Background(), "track123")
	if err != nil {
		t.Fatalf("Get (cached) returned error: %v", err)
	}
	if af2 != af {
		t.Error("expected same pointer from cache")
	}

	if c := atomic.LoadInt32(&callCount); c != 1 {
		t.Errorf("expected still 1 API call after cache hit, got %d", c)
	}
}

func TestFeatureCacheGetAPIError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/audio-features/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"internal error"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSourceWithClient(srv.URL)

	af, err := ps.features.Get(context.Background(), "badtrack")
	if err == nil {
		t.Fatal("expected error from API")
	}
	if af != nil {
		t.Errorf("expected nil AudioFeatures on error, got %+v", af)
	}

	// Verify nothing was cached (error should not be cached)
	ps.features.mu.Lock()
	_, inCache := ps.features.cache["badtrack"]
	_, inFlight := ps.features.inflight["badtrack"]
	ps.features.mu.Unlock()

	if inCache {
		t.Error("errored trackID should not be in cache")
	}
	if inFlight {
		t.Error("errored trackID should not be in inflight")
	}
}

// ---------------------------------------------------------------------------
// AudioFeatures through PlayerSource method
// ---------------------------------------------------------------------------

func TestAudioFeaturesViaPlayerSource(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/audio-features/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"audio_features": [
				{
					"energy": 0.5,
					"valence": 0.5,
					"danceability": 0.5,
					"tempo": 100.0,
					"acousticness": 0.5,
					"id": "af-track"
				}
			]
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSourceWithClient(srv.URL)

	af, err := ps.AudioFeatures(context.Background(), "af-track")
	if err != nil {
		t.Fatalf("AudioFeatures returned error: %v", err)
	}
	if af == nil {
		t.Fatal("expected non-nil AudioFeatures")
	}
	if af.Tempo != 100.0 {
		t.Errorf("Tempo = %f, want 100.0", af.Tempo)
	}
}

// ---------------------------------------------------------------------------
// GetAlbum error handling
// ---------------------------------------------------------------------------

func TestGetAlbumError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/albums/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":"not found"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	_, err := ps.GetAlbum(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for missing album")
	}
}

// ---------------------------------------------------------------------------
// GetArtist error handling
// ---------------------------------------------------------------------------

func TestGetArtistError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/artists/bad", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":"not found"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	_, err := ps.GetArtist(context.Background(), "bad")
	if err == nil {
		t.Fatal("expected error for missing artist")
	}
}

// ---------------------------------------------------------------------------
// RecentlyPlayed error handling
// ---------------------------------------------------------------------------

func TestRecentlyPlayedError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/me/player/recently-played", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"server error"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	_, err := ps.RecentlyPlayed(context.Background())
	if err == nil {
		t.Fatal("expected error for server failure")
	}
}

// ---------------------------------------------------------------------------
// Playlists with liked songs error (non-fatal)
// ---------------------------------------------------------------------------

func TestPlaylistsLikedSongsError(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/me/tracks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"server error"}`)
	})

	mux.HandleFunc("/v1/me/playlists", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items": []}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	playlists, err := ps.Playlists(context.Background())
	if err != nil {
		t.Fatalf("Playlists should not fail when liked songs count fails: %v", err)
	}

	// Should still have the liked songs entry with 0 count
	if len(playlists) != 1 {
		t.Fatalf("expected 1 playlist (liked), got %d", len(playlists))
	}
	if playlists[0].ID != LikedPlaylistID {
		t.Errorf("first playlist ID = %q, want %q", playlists[0].ID, LikedPlaylistID)
	}
	if playlists[0].TrackCount != 0 {
		t.Errorf("liked TrackCount = %d, want 0 (error fallback)", playlists[0].TrackCount)
	}
}

// ---------------------------------------------------------------------------
// GetArtist with no images / no genres
// ---------------------------------------------------------------------------

func TestGetArtistMinimalFields(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/artists/art-min", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "art-min",
			"name": "Minimal Artist",
			"genres": [],
			"images": []
		}`)
	})

	mux.HandleFunc("/v1/artists/art-min/top-tracks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"tracks": []}`)
	})

	mux.HandleFunc("/v1/artists/art-min/albums", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items": []}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	page, err := ps.GetArtist(context.Background(), "art-min")
	if err != nil {
		t.Fatalf("GetArtist returned error: %v", err)
	}

	if page.Name != "Minimal Artist" {
		t.Errorf("Name = %q, want %q", page.Name, "Minimal Artist")
	}
	if page.ImageURL != "" {
		t.Errorf("ImageURL = %q, want empty", page.ImageURL)
	}
	if len(page.Genres) != 0 {
		t.Errorf("expected 0 genres, got %d", len(page.Genres))
	}
	if len(page.Tracks) != 0 {
		t.Errorf("expected 0 tracks, got %d", len(page.Tracks))
	}
	if len(page.Albums) != 0 {
		t.Errorf("expected 0 albums, got %d", len(page.Albums))
	}
}

// ---------------------------------------------------------------------------
// GetAlbum with no artists / no images
// ---------------------------------------------------------------------------

func TestGetAlbumMinimalFields(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/v1/albums/alb-min", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"id": "alb-min",
			"uri": "spotify:album:alb-min",
			"name": "Minimal Album",
			"release_date": "2023",
			"artists": [],
			"images": [],
			"tracks": {"items": []}
		}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	page, err := ps.GetAlbum(context.Background(), "alb-min")
	if err != nil {
		t.Fatalf("GetAlbum returned error: %v", err)
	}

	if page.Artist != "" {
		t.Errorf("Artist = %q, want empty", page.Artist)
	}
	if page.ImageURL != "" {
		t.Errorf("ImageURL = %q, want empty", page.ImageURL)
	}
	if page.Year != "2023" {
		t.Errorf("Year = %q, want %q", page.Year, "2023")
	}
	if len(page.Tracks) != 0 {
		t.Errorf("expected 0 tracks, got %d", len(page.Tracks))
	}
}

// ---------------------------------------------------------------------------
// Search with nil result sections
// ---------------------------------------------------------------------------

func TestSearchNilSections(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return only tracks, no artists or albums keys at all
		fmt.Fprint(w, `{"tracks": {"items": []}}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSourceWithClient(srv.URL)

	results, err := ps.Search(context.Background(), "test")
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	// Should not panic on nil Artists/Albums
	if results == nil {
		t.Fatal("expected non-nil results")
	}
}

// ---------------------------------------------------------------------------
// PlaylistTracksPage error handling
// ---------------------------------------------------------------------------

func TestPlaylistTracksPageError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/playlists/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		fmt.Fprint(w, `{"error":"not found"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	_, _, err := ps.PlaylistTracksPage(context.Background(), "nonexistent", 0, 50)
	if err == nil {
		t.Fatal("expected error for missing playlist")
	}
}

// ---------------------------------------------------------------------------
// LikedTracksPage error handling
// ---------------------------------------------------------------------------

func TestLikedTracksPageError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/me/tracks", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, `{"error":"server error"}`)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ps := newTestPlayerSource(srv.URL)

	_, _, err := ps.LikedTracksPage(context.Background(), 0, 50)
	if err == nil {
		t.Fatal("expected error for server failure")
	}
}
