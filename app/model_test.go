package app

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danielfry/spotui/mood"
	"github.com/danielfry/spotui/source"
)

type fakeSource struct{ track *source.Track }

func (f *fakeSource) CurrentTrack() (*source.Track, error)  { return f.track, nil }
func (f *fakeSource) Play() error                          { return nil }
func (f *fakeSource) Seek(position time.Duration) error    { return nil }
func (f *fakeSource) Pause() error                         { return nil }
func (f *fakeSource) Next() error                          { return nil }
func (f *fakeSource) Previous() error                      { return nil }

func TestModelInitialState(t *testing.T) {
	m := NewModel(&fakeSource{})
	if m.mood.Name != "idle" {
		t.Errorf("initial mood = %q, want %q", m.mood.Name, "idle")
	}
	if m.track != nil {
		t.Error("expected nil track initially")
	}
}

func TestModelTrackUpdate(t *testing.T) {
	m := NewModel(&fakeSource{})
	track := &source.Track{Name: "Holocene", Artist: "Bon Iver", Album: "Bon Iver", Playing: true, Duration: 5 * time.Minute}
	updated, _ := m.Update(trackUpdateMsg{track})
	m = updated.(Model)
	if m.track == nil {
		t.Fatal("expected track to be set")
	}
	if m.track.Name != "Holocene" {
		t.Errorf("track name = %q, want %q", m.track.Name, "Holocene")
	}
}

func TestModelTrackClearedOnNil(t *testing.T) {
	m := NewModel(&fakeSource{})
	m.track = &source.Track{Name: "Test", Artist: "Test", Playing: true}
	m.mood = mood.Warm
	updated, _ := m.Update(trackUpdateMsg{nil})
	m = updated.(Model)
	if m.track != nil {
		t.Error("expected track to be nil after nil update")
	}
}

func TestModelQuit(t *testing.T) {
	m := NewModel(&fakeSource{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Error("expected quit command")
	}
}

func TestModelHelpToggle(t *testing.T) {
	m := NewModel(&fakeSource{})
	if m.showHelp {
		t.Error("expected showHelp to be false initially")
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if !m.showHelp {
		t.Error("expected showHelp to be true after ?")
	}
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = updated.(Model)
	if m.showHelp {
		t.Error("expected showHelp to be false after second ?")
	}
}
