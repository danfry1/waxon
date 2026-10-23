package app

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/source"
)

func runeKey(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }

func TestVolumeKeysDebounce(t *testing.T) {
	var sent []int
	stub := &StubSource{SetVolumeFn: func(_ context.Context, v int) error { sent = append(sent, v); return nil }}
	m := newTestModel(stub)
	m.volume = 50

	var cmds []tea.Cmd
	for _, k := range []string{"+", "+", "="} {
		result, cmd := m.Update(runeKey(k))
		m = result.(Model)
		cmds = append(cmds, cmd)
	}
	if m.volume != 65 {
		t.Fatalf("volume = %d, want 65 shown immediately", m.volume)
	}
	// Only the commit for the last press goes out.
	for _, c := range cmds {
		msg, ok := c().(volumeCommitMsg)
		if !ok {
			t.Fatalf("expected volumeCommitMsg, got %T", msg)
		}
		result, cmd := m.Update(msg)
		m = result.(Model)
		runCmdTree(cmd)
	}
	if len(sent) != 1 || sent[0] != 65 {
		t.Errorf("SetVolume calls = %v, want [65]", sent)
	}
}

func TestVolumeClampsAndHoldsAgainstPoll(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.volume = 3
	result, _ := m.Update(runeKey("-"))
	m = result.(Model)
	if m.volume != 0 {
		t.Errorf("volume = %d, want clamped to 0", m.volume)
	}
	// A poll that started before the change reports the old level.
	result, _ = m.Update(trackUpdateMsg{track: &source.Track{ID: "t"}, volume: 3})
	if got := result.(Model).volume; got != 0 {
		t.Errorf("stale poll overwrote volume: %d", got)
	}
}

func TestRepeatKeyCycles(t *testing.T) {
	var modes []source.RepeatMode
	stub := &StubSource{SetRepeatFn: func(_ context.Context, r source.RepeatMode) error { modes = append(modes, r); return nil }}
	m := newTestModel(stub)
	for range 3 {
		result, cmd := m.Update(runeKey("r"))
		m = result.(Model)
		runCmdTree(cmd)
	}
	want := []source.RepeatMode{source.RepeatContext, source.RepeatTrack, source.RepeatOff}
	if len(modes) != 3 || modes[0] != want[0] || modes[1] != want[1] || modes[2] != want[2] {
		t.Errorf("repeat cycle = %v, want %v", modes, want)
	}
}

func TestShuffleKeyInNowPlaying(t *testing.T) {
	var got []bool
	stub := &StubSource{SetShuffleFn: func(_ context.Context, on bool) error { got = append(got, on); return nil }}
	m := newTestModel(stub)
	m.mode = ModeNowPlaying
	result, cmd := m.Update(runeKey("S"))
	runCmdTree(cmd)
	if !result.(Model).shuffleOn || len(got) != 1 || !got[0] {
		t.Errorf("S in Now Playing should turn shuffle on; calls=%v", got)
	}
}

func TestGRStillOpensRecent(t *testing.T) {
	m := newTestModel(&StubSource{})
	result, _ := m.Update(runeKey("g"))
	result, _ = result.(Model).Update(runeKey("r"))
	if got := result.(Model).tracklist.title; got != "Recently Played" {
		t.Errorf("gr should still open recently played; title=%q", got)
	}
}
