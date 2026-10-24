package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/danfry1/waxon/source"
)

// An artist page that fails to load returns to the view it was opened from,
// instead of leaving a loading state that never ends.
func TestFailedArtistLoadRestoresPreviousView(t *testing.T) {
	stub := &StubSource{
		GetArtistFn: func(context.Context, string) (*source.ArtistPage, error) {
			return nil, errors.New("spotify: HTTP 503: Service Unavailable")
		},
	}
	m := newTestModel(stub)
	m.tracklist.SetTracks([]source.Track{{ID: "t1", Name: "Song"}}, "Liked Songs", "uri")

	m, cmd := m.executeAction(ActionItem{Type: ActionGoArtist}, actionSubject{artistID: "a1"})
	if !m.tracklist.loading {
		t.Fatal("the artist should be loading")
	}
	res, _ := m.Update(cmd())
	m = res.(Model)
	if m.tracklist.loading || m.tracklist.title != "Liked Songs" || len(m.tracklist.tracks) != 1 {
		t.Errorf("after the failure: loading=%v title=%q tracks=%d, want Liked Songs back",
			m.tracklist.loading, m.tracklist.title, len(m.tracklist.tracks))
	}
	if !m.toast.Visible() || !strings.Contains(m.toast.message, "artist") {
		t.Errorf("toast = %q, want it to say the artist didn't load", m.toast.message)
	}
}

// A failure for a page the user has already left doesn't move them.
func TestStaleArtistFailureLeavesViewAlone(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "t1"}}, "Elsewhere", "uri")
	res, _ := m.Update(pageLoadFailedMsg{"artist", errors.New("boom")})
	if got := res.(Model).tracklist.title; got != "Elsewhere" {
		t.Errorf("title = %q, want the view left alone", got)
	}
}

// The view loading doesn't show the last one's header lines.
func TestLoadingClearsPreviousHeader(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetTracks([]source.Track{{ID: "t1"}}, "Liked Songs", "uri")
	m.tracklist.SetHeaderInfo("50 of 2714 tracks")
	m.tracklist.SetSubtitle("indie")
	m.tracklist.SetLoading("Loading artist...")
	if view := ansi.Strip(m.tracklist.View(true)); strings.Contains(view, "2714") || strings.Contains(view, "indie") {
		t.Errorf("the loading view still shows the old header:\n%s", view)
	}
}
