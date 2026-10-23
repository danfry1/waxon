package app

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/danfry1/waxon/source"
)

func TestProgressTickStopsWhenIdle(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.track = &source.Track{ID: "t1", Playing: false, Duration: time.Minute}

	result, cmd := m.Update(progressTickMsg(time.Now()))
	model := result.(Model)
	if model.progressTicking {
		t.Error("tick should stop while paused with nothing loading")
	}
	if cmd != nil {
		t.Error("no further tick should be scheduled while idle")
	}
}

func TestProgressTickRestartsOnPlayback(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.progressTicking = false

	result, cmd := m.Update(trackUpdateMsg{track: &source.Track{ID: "t1", Playing: true, Duration: time.Minute}})
	model := result.(Model)
	if !model.progressTicking || cmd == nil {
		t.Error("playback starting should restart the progress tick")
	}

	// A second message while the tick is live must not start another chain.
	result, _ = model.Update(noopMsg{})
	if !result.(Model).progressTicking {
		t.Error("progressTicking should stay set")
	}
}

type noopMsg struct{}

func TestProgressTickKeepsSpinnerAlive(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.tracklist.SetLoading("PL")
	result, cmd := m.Update(progressTickMsg(time.Now()))
	if !result.(Model).progressTicking || cmd == nil {
		t.Error("a loading spinner needs the tick even with nothing playing")
	}
}

func TestVinylHeldWhileBlurred(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeNowPlaying
	m.vinylMode = true
	m.blurred = true
	m.track = &source.Track{ID: "t1", Playing: true, Duration: time.Minute}
	result, _ := m.Update(progressTickMsg(time.Now()))
	if result.(Model).vinylAngle != 0 {
		t.Error("vinyl should not spin while the terminal is unfocused")
	}
}

func TestVinylFrameMemoised(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	vinylFrame(img, 0.5, 8, 4, nil)
	vinylMemo.out = "sentinel"
	if b := vinylFrame(img, 0.5, 8, 4, nil); b != "sentinel" {
		t.Error("same inputs should reuse the memoised frame")
	}
	if c := vinylFrame(img, 0.6, 8, 4, nil); c == "sentinel" || c == "" {
		t.Error("a new angle should re-render")
	}
}

func TestPrefetchNextOncePerTrack(t *testing.T) {
	calls := 0
	stub := &StubSource{QueueFn: func(context.Context) ([]source.Track, error) {
		calls++
		return []source.Track{{ID: "next"}}, nil
	}}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t1", Playing: true, Duration: 3 * time.Minute, Position: time.Minute}
	if cmd := m.maybePrefetchNext(); cmd != nil {
		t.Fatal("no prefetch until the track is near its end")
	}
	m.track.Position = 3*time.Minute - 10*time.Second
	cmd := m.maybePrefetchNext()
	if cmd == nil {
		t.Fatal("expected a prefetch near the end of the track")
	}
	if _, ok := cmd().(queueLoadedMsg); !ok || calls != 1 {
		t.Errorf("prefetch should load the queue once; calls=%d", calls)
	}
	if again := m.maybePrefetchNext(); again != nil {
		t.Error("prefetch should run once per track")
	}
}

func TestLyricsLineFollowsPositionWhilePaused(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.mode = ModeNowPlaying
	m.lyricsView = true
	m.track = &source.Track{ID: "t1", Playing: false, Position: 25 * time.Second, Duration: time.Minute}
	lines := []source.LyricLine{{Time: 0, Text: "a"}, {Time: 10 * time.Second, Text: "b"}, {Time: 20 * time.Second, Text: "c"}}
	result, _ := m.Update(lyricsLoadedMsg{trackID: "t1", lyrics: &source.Lyrics{Synced: true, Lines: lines}})
	if got := result.(Model).lyricsLine; got != 2 {
		t.Errorf("lyricsLine = %d, want 2 (lyrics loaded while paused at 0:25)", got)
	}
}

func TestPrefetchRateLimitStartsCooldown(t *testing.T) {
	stub := &StubSource{QueueFn: func(context.Context) ([]source.Track, error) {
		return nil, rateLimitErr{retry: 5}
	}}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t1", Playing: true, Duration: time.Minute, Position: 50 * time.Second}
	cmd := m.maybePrefetchNext()
	if _, ok := cmd().(trackErrorMsg); !ok {
		t.Error("a 429 on prefetch should surface as trackErrorMsg so the cooldown starts")
	}
}
