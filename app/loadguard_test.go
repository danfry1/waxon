package app

import (
	"context"
	"errors"
	"testing"

	"github.com/danfry1/waxon/source"
)

// Edit A, then open B (slow, live) before the 4s refresh fires.
func TestEditRefreshDoesNotTakeOverPlaylistOpenedSince(t *testing.T) {
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, id string, _, _ int) ([]source.Track, int, error) {
			return []source.Track{{ID: "from-" + id}}, 1, nil
		},
	}
	m := newTestModel(stub)
	a := source.Playlist{ID: "a", URI: "spotify:playlist:a", Name: "A"}
	b := source.Playlist{ID: "b", URI: "spotify:playlist:b", Name: "B"}
	m.playlists = []source.Playlist{a, b}
	m.tracklist.SetLoadingFor(a.Name, a.URI)
	m = pumpModel(m, m.livePlaylistPage(a, false))
	m.pendingViewRefresh = "a"
	// user opens B
	m.tracklist.SetLoadingFor(b.Name, b.URI)
	slowB := m.livePlaylistPage(b, false)
	res, cmd := m.Update(refreshPlaylistsMsg{})
	m = res.(Model)
	m = pumpModel(m, slowB)
	m = pumpModel(m, cmd)
	if m.tracklist.contextURI != b.URI {
		t.Errorf("opened B but view shows %s", m.tracklist.contextURI)
	}
}

// Open B (slow) from A, go back to A, then B lands.
func TestBackThenSlowPageKeepsRestoredView(t *testing.T) {
	stub := &StubSource{
		PlaylistTracksPageFn: func(_ context.Context, id string, _, _ int) ([]source.Track, int, error) {
			return []source.Track{{ID: "from-" + id}}, 1, nil
		},
	}
	m := newTestModel(stub)
	a := source.Playlist{ID: "a", URI: "spotify:playlist:a", Name: "A"}
	b := source.Playlist{ID: "b", URI: "spotify:playlist:b", Name: "B"}
	m.playlists = []source.Playlist{a, b}
	m.tracklist.SetLoadingFor(a.Name, a.URI)
	m = pumpModel(m, m.livePlaylistPage(a, false))
	m.pushNav()
	m.tracklist.SetLoadingFor(b.Name, b.URI)
	slowB := m.livePlaylistPage(b, false)
	m.popNav()
	m = pumpModel(m, slowB)
	if m.tracklist.contextURI != a.URI {
		t.Errorf("went back to A but B replaced it")
	}
}

// A player that stopped for a reason other than its login restarts from
// the device picker without signing in again.
func TestPlayHereRestartsSignedInPlayer(t *testing.T) {
	started := 0
	deps := SetupDeps{Player: &fakePlayer{installed: true, loggedIn: true}}
	m := newTestModel(&StubSource{}).WithOptions(Options{
		PlayerSetup: &deps,
		StartPlayer: func() string { started++; return "waxon (box)" },
	})
	m.playerErr = errors.New("librespot stopped: audio device unavailable")
	m, _ = m.openPlayerSetup()
	if m.mode == ModeSetup || started != 1 || m.opts.PlayerName != "waxon (box)" {
		t.Errorf("mode=%v started=%d player=%q; want a direct restart", m.mode, started, m.opts.PlayerName)
	}
}

func TestPlayHereSignsInAgainAfterLoginFailure(t *testing.T) {
	deps := SetupDeps{Player: &fakePlayer{installed: true, loggedIn: true}}
	m := newTestModel(&StubSource{}).WithOptions(Options{PlayerSetup: &deps, StartPlayer: func() string { return "x" }})
	m.playerErr = errors.New("librespot stopped: Login failed with reason: Bad credentials")
	m, _ = m.openPlayerSetup()
	if m.mode != ModeSetup {
		t.Errorf("a login failure should go through the player sign-in, mode=%v", m.mode)
	}
}
