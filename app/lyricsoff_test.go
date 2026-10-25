package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/danfry1/waxon/lyrics"
	"github.com/danfry1/waxon/source"
)

func lyricsStub(err error, asked *int) *StubSource {
	return &StubSource{LyricsFn: func(context.Context, source.Track) (*source.Lyrics, error) {
		*asked++
		return nil, err
	}}
}

// With lyrics off, lrclib.net is never asked, whatever the user does.
func TestLyricsOffNeverAsks(t *testing.T) {
	asked := 0
	m := newTestModel(lyricsStub(nil, &asked)).WithOptions(Options{LyricsOff: true})
	m.track = &source.Track{ID: "t1", Name: "Song"}
	if cmd := m.ensureLyrics(); cmd != nil {
		cmd()
	}
	m.mode = ModeNowPlaying
	res, cmd := m.Update(runeKey("l"))
	if cmd != nil {
		cmd()
	}
	if asked != 0 {
		t.Errorf("lrclib was asked %d times with lyrics off", asked)
	}
	_ = res
}

// Once lrclib is unreachable (a proxy blocking it), track changes stop
// asking; pressing l asks again.
func TestUnreachableLyricsStopAsking(t *testing.T) {
	asked := 0
	blocked := fmt.Errorf("%w: lrclib status 403", lyrics.ErrUnreachable)
	m := newTestModel(lyricsStub(blocked, &asked))
	m.track = &source.Track{ID: "t1", Name: "One"}
	res, _ := m.Update(m.fetchLyrics(*m.track)())
	m = res.(Model)
	if !m.lyricsBlocked() {
		t.Fatal("a proxy refusal should leave lrclib alone for a while")
	}
	m.track = &source.Track{ID: "t2", Name: "Two"}
	if cmd := m.ensureLyrics(); cmd != nil {
		t.Error("the next track shouldn't ask a blocked lrclib again")
	}
	if asked != 1 {
		t.Errorf("asked %d times, want once", asked)
	}
	m.mode = ModeNowPlaying
	m.lyricsView = false
	res, cmd := m.Update(runeKey("l"))
	if cmd == nil {
		t.Fatal("l should try again")
	}
	cmd()
	if asked != 2 || res.(Model).lyricsBlocked() {
		t.Errorf("after l: asked %d, blocked %v", asked, res.(Model).lyricsBlocked())
	}
}

// "l to try again" is shown with the lyrics open: l there retries and
// keeps them open, rather than closing them.
func TestRetryWithLyricsOpen(t *testing.T) {
	asked := 0
	m := newTestModel(lyricsStub(nil, &asked))
	m.track = &source.Track{ID: "t1", Name: "One"}
	m.mode, m.lyricsView = ModeNowPlaying, true
	m.lyricsTrackID = "t1"
	m.lyricsBlockedUntil = time.Now().Add(time.Minute)
	res, cmd := m.Update(runeKey("l"))
	if cmd == nil || !res.(Model).lyricsView {
		t.Fatalf("l with the blocked message showing: cmd %v, view %v", cmd != nil, res.(Model).lyricsView)
	}
	cmd()
	if asked != 1 {
		t.Errorf("asked %d times, want once", asked)
	}
}

// The wait after a block ends by itself: a network that changes gets
// lyrics back without a restart.
func TestBlockedLyricsComeBack(t *testing.T) {
	asked := 0
	m := newTestModel(lyricsStub(nil, &asked))
	m.track = &source.Track{ID: "t2", Name: "Two"}
	m.lyricsBlockedUntil = time.Now().Add(-time.Second) // the wait is over
	if cmd := m.ensureLyrics(); cmd == nil {
		t.Fatal("lyrics should be asked for again once the wait is over")
	}
}
