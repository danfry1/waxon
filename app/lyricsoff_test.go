package app

import (
	"context"
	"fmt"
	"testing"

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
	if !m.lyricsUnreachable {
		t.Fatal("a proxy refusal should mark lrclib unreachable")
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
	if asked != 2 || res.(Model).lyricsUnreachable {
		t.Errorf("after l: asked %d, unreachable %v", asked, res.(Model).lyricsUnreachable)
	}
}
