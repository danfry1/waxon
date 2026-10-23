package spotify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPlayingItemEpisode(t *testing.T) {
	raw := json.RawMessage(`{"id":"e1","uri":"spotify:episode:e1","name":"Ep","duration_ms":60000,
		"images":[{"url":"http://img","width":640}],"show":{"id":"sh1","name":"Sleeve Notes","publisher":"Waxon Radio"}}`)
	tr, ok := playingItem("episode", raw)
	if !ok || !tr.IsEpisode || tr.ID != "spotify:episode:e1" || tr.Artist != "Sleeve Notes" || tr.ShowID != "sh1" || tr.ArtworkURL == "" {
		t.Errorf("episode = %+v ok=%v", tr, ok)
	}
	if _, adOK := playingItem("ad", json.RawMessage(`{}`)); adOK {
		t.Error("ads have nothing to show")
	}
	tr, ok = playingItem("track", json.RawMessage(`{"id":"t1","uri":"spotify:track:t1","name":"Song"}`))
	if !ok || tr.IsEpisode || tr.ID != "t1" {
		t.Errorf("track = %+v", tr)
	}
}

func TestItemURI(t *testing.T) {
	if itemURI("abc") != "spotify:track:abc" || itemURI("spotify:episode:x") != "spotify:episode:x" {
		t.Error("bare IDs are tracks; URIs pass through")
	}
}

func TestPlaylistItemEpisodeIsEpisode(t *testing.T) {
	tr := apiTrackToSource(apiTrack{ID: "e1", URI: "spotify:episode:e1", Name: "Ep", Show: &apiShowRef{ID: "sh", Name: "Show"}})
	if !tr.IsEpisode || tr.ID != "spotify:episode:e1" || tr.Artist != "Show" {
		t.Errorf("playlist episode = %+v", tr)
	}
}

func TestQueueKeepsEpisodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/me/player/queue") {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"queue":[
			{"type":"track","id":"t1","uri":"spotify:track:t1","name":"Song"},
			{"type":"episode","id":"e1","uri":"spotify:episode:e1","name":"Ep","show":{"id":"sh","name":"Show"}}]}`))
	}))
	defer srv.Close()
	got, err := newTestPlayerSource(srv.URL).Queue(context.Background())
	if err != nil || len(got) != 2 {
		t.Fatalf("queue = %+v, %v", got, err)
	}
	if got[0].ID != "t1" || !got[1].IsEpisode || got[1].ID != "spotify:episode:e1" {
		t.Errorf("queue = %+v", got)
	}
}

func TestShowEpisodesKeepNullSlots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"total":3,"items":[{"id":"e1","uri":"spotify:episode:e1","name":"A"},null,{"id":"e3","uri":"spotify:episode:e3","name":"C"}]}`))
	}))
	defer srv.Close()
	got, _, err := newTestPlayerSource(srv.URL).ShowEpisodesPage(context.Background(), "sh", 0, 50)
	if err != nil || len(got) != 3 || !got[1].IsSeparator {
		t.Errorf("an unavailable episode should keep its slot so offsets line up: %+v %v", got, err)
	}
}
