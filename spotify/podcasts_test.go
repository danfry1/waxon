package spotify

import (
	"encoding/json"
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
