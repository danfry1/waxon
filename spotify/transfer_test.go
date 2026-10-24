package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	spotifyapi "github.com/zmb3/spotify/v2"

	"github.com/danfry1/waxon/source"
)

// When Spotify can't hand playback over (a server error, as from librespot
// to the Spotify app), the same track is started on the device instead, in
// the same context and from the same point.
func TestTransferFallsBackToPlayingOnTheDevice(t *testing.T) {
	for _, refusal := range []struct {
		status int
		body   string
	}{
		{http.StatusInternalServerError, `{"error":{"status":500,"message":"Server error."}}`},
		{http.StatusForbidden, `{"error":{"status":403,"message":"Player command failed: Restriction violated","reason":"UNKNOWN"}}`},
	} {
		t.Run(http.StatusText(refusal.status), func(t *testing.T) { testTransferFallback(t, refusal.status, refusal.body) })
	}
}

func testTransferFallback(t *testing.T, status int, body string) {
	var played map[string]any
	var playedOn string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/v1/me/player":
			w.WriteHeader(status)
			_, _ = io.WriteString(w, body)
		case r.Method == http.MethodGet && r.URL.Path == "/v1/me/player":
			_, _ = io.WriteString(w, `{"progress_ms":113974,"is_playing":true,
				"context":{"uri":"spotify:user:me:collection"},
				"item":{"uri":"spotify:track:42x","id":"42x","name":"Song"},
				"device":{"id":"lib","name":"waxon (Mac)"}}`)
		case r.Method == http.MethodPut && r.URL.Path == "/v1/me/player/play":
			playedOn = r.URL.Query().Get("device_id")
			_ = json.NewDecoder(r.Body).Decode(&played)
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	hc := &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, testURL: srv.URL}}
	p := &PlayerSource{client: spotifyapi.New(hc), httpClient: hc}

	if err := p.TransferPlayback(context.Background(), "app"); err != nil {
		t.Fatalf("TransferPlayback = %v, want the fallback to succeed", err)
	}
	if playedOn != "app" || played["context_uri"] != "spotify:user:me:collection" ||
		played["position_ms"] != float64(113974) {
		t.Errorf("played on %q with %v", playedOn, played)
	}
	if off, _ := played["offset"].(map[string]any); off["uri"] != "spotify:track:42x" {
		t.Errorf("offset = %v, want the playing track", played["offset"])
	}
}

// With nothing playing there's nothing to start: the transfer's own error
// comes back.
func TestTransferFallbackNeedsSomethingPlaying(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, `{"error":{"status":502,"message":"Bad gateway."}}`)
			return
		}
		w.WriteHeader(http.StatusNoContent) // GET /me/player: nothing playing
	}))
	defer srv.Close()
	hc := &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, testURL: srv.URL}}
	p := &PlayerSource{client: spotifyapi.New(hc), httpClient: hc}
	if err := p.TransferPlayback(context.Background(), "app"); !errors.Is(err, source.ErrDeviceUnavailable) {
		t.Errorf("err = %v, want ErrDeviceUnavailable", err)
	}
}

// Liked Songs is played as the account's collection: the Spotify apps
// refuse spotify:collection:tracks.
func TestPlayLikedSongsAsTheAccountsCollection(t *testing.T) {
	var sent []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/me":
			_, _ = io.WriteString(w, `{"id":"dan"}`)
		case "/v1/me/player/play":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			sent = append(sent, body["context_uri"].(string))
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()
	hc := &http.Client{Transport: &rewriteTransport{base: http.DefaultTransport, testURL: srv.URL}}
	p := &PlayerSource{client: spotifyapi.New(hc), httpClient: hc}
	ctx := context.Background()
	for _, c := range []string{likedSongsURI, "spotify:playlist:abc"} {
		if err := p.PlayTrack(ctx, c, "spotify:track:x"); err != nil {
			t.Fatal(err)
		}
	}
	if len(sent) != 2 || sent[0] != "spotify:user:dan:collection" || sent[1] != "spotify:playlist:abc" {
		t.Errorf("contexts sent = %v", sent)
	}
}
