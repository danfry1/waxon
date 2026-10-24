package app

import (
	"context"
	"image"
	"sync"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/media"
	"github.com/danfry1/waxon/source"
)

type fakeMedia struct {
	mu      sync.Mutex
	updates []media.State
}

func (f *fakeMedia) Update(s media.State) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updates = append(f.updates, s)
}
func (f *fakeMedia) Close() {}

func (f *fakeMedia) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.updates)
}

func (f *fakeMedia) last() media.State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.updates[len(f.updates)-1]
}

// Media keys run the same commands as waxon's own keys.
func TestMediaKeysControlPlayback(t *testing.T) {
	var calls []string
	var seekedTo time.Duration
	rec := func(name string) func(context.Context) error {
		return func(context.Context) error { calls = append(calls, name); return nil }
	}
	stub := &StubSource{
		PlayFn: rec("play"), PauseFn: rec("pause"), NextFn: rec("next"), PreviousFn: rec("previous"),
		SeekFn: func(_ context.Context, d time.Duration) error {
			seekedTo = d
			calls = append(calls, "seek")
			return nil
		},
	}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t", Playing: true, Duration: 3 * time.Minute}
	for _, e := range []media.Event{
		{Command: media.Toggle}, // playing → pause
		{Command: media.Next},
		{Command: media.Previous},
		{Command: media.Play},
		{Command: media.Seek, Position: 42 * time.Second},
	} {
		_, cmd := m.Update(MediaMsg(e))
		if cmd != nil {
			cmd()
		}
	}
	want := []string{"pause", "next", "previous", "play", "seek"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %v, want %v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Errorf("call %d = %s, want %s", i, calls[i], want[i])
		}
	}
	if seekedTo != 42*time.Second {
		t.Errorf("seeked to %v, want 42s", seekedTo)
	}
}

func TestNowPlayingPublishedOnlyWhenItChanges(t *testing.T) {
	fake := &fakeMedia{}
	m := newTestModel(&StubSource{}).WithOptions(Options{Media: fake})
	now := time.Now()
	m.track = &source.Track{ID: "a", Name: "Song", Artist: "Band", Album: "LP", Duration: 4 * time.Minute, Position: 10 * time.Second, Playing: true}
	m.publishMedia(now)
	if fake.count() != 1 || fake.last().Title != "Song" || !fake.last().Playing {
		t.Fatalf("first publish: %+v", fake.updates)
	}
	// Playing on as expected: the system advances the position itself.
	m.track.Position += 5 * time.Second
	m.publishMedia(now.Add(5 * time.Second))
	if fake.count() != 1 {
		t.Errorf("an unchanged track was published again (%d updates)", fake.count())
	}
	// A seek is a jump the system can't know about.
	m.track.Position = 2 * time.Minute
	m.publishMedia(now.Add(6 * time.Second))
	if fake.count() != 2 || fake.last().Position != 2*time.Minute {
		t.Errorf("a seek should be published, got %d updates", fake.count())
	}
	m.track.Playing = false
	m.publishMedia(now.Add(7 * time.Second))
	if fake.count() != 3 || fake.last().Playing {
		t.Error("pausing should be published")
	}
	m.track = &source.Track{ID: "b", Name: "Next one"}
	m.publishMedia(now.Add(8 * time.Second))
	if fake.count() != 4 || fake.last().Title != "Next one" {
		t.Error("a new track should be published")
	}
	m.track = nil
	m.publishMedia(now.Add(9 * time.Second))
	if fake.count() != 5 || !fake.last().Empty {
		t.Error("nothing playing should clear the system's display")
	}
}

// Local files have no Spotify id: they're still told apart and cleared.
func TestNowPlayingHandlesLocalFiles(t *testing.T) {
	fake := &fakeMedia{}
	m := newTestModel(&StubSource{}).WithOptions(Options{Media: fake})
	now := time.Now()
	m.track = &source.Track{URI: "spotify:local:A:B:One:180", Name: "One", Duration: 3 * time.Minute, Playing: true}
	m.publishMedia(now)
	m.track = &source.Track{URI: "spotify:local:A:B:Two:180", Name: "Two", Duration: 3 * time.Minute, Playing: true}
	m.publishMedia(now)
	if fake.count() != 2 || fake.last().Title != "Two" {
		t.Fatalf("the second local file wasn't published: %d updates", fake.count())
	}
	m.track = nil
	m.publishMedia(now)
	if fake.count() != 3 || !fake.last().Empty {
		t.Error("a local file stopping should clear the system's display")
	}
}

func TestNowPlayingSendsCoverWhenItLoads(t *testing.T) {
	fake := &fakeMedia{}
	m := newTestModel(&StubSource{}).WithOptions(Options{Media: fake})
	m.track = &source.Track{ID: "a", Name: "Song", ArtworkURL: "https://img/a", Playing: true}
	res, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40}) // any update publishes
	m = res.(Model)
	if fake.last().Artwork != nil {
		t.Fatal("no cover has loaded yet")
	}
	cover := image.NewRGBA(image.Rect(0, 0, 4, 4))
	res, _ = m.Update(artworkLoadedMsg{url: "https://img/a", img: cover})
	m = res.(Model)
	if got := fake.last(); got.Artwork == nil || got.ArtworkURL != "https://img/a" {
		t.Errorf("the cover should be sent once it loads: %+v", got)
	}
}
