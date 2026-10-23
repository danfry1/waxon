package app

import (
	"testing"

	"github.com/danfry1/waxon/source"
)

func countModel(n int) Model {
	m := newTestModel(&StubSource{})
	m.focusPane = PaneTrackList
	tracks := make([]source.Track, n)
	for i := range tracks {
		tracks[i] = source.Track{ID: string(rune('A' + i)), Name: "t"}
	}
	m.tracklist.SetTracks(tracks, "PL", "")
	return m
}

func TestCountMotions(t *testing.T) {
	for _, tc := range []struct {
		keys []string
		want int
	}{
		{[]string{"5", "j"}, 5},
		{[]string{"1", "2", "j"}, 12},
		{[]string{"5", "j", "2", "k"}, 3},
		{[]string{"7", "G"}, 6},
		{[]string{"1", "0", "g", "g"}, 9},
		{[]string{"j", "j", "0"}, 2},  // a lone 0 is not a count
		{[]string{"9", "9", "j"}, 19}, // clamped to the last row
	} {
		m := countModel(20)
		m, _ = press(t, m, tc.keys...)
		if got := m.tracklist.Cursor(); got != tc.want {
			t.Errorf("%v: cursor = %d, want %d", tc.keys, got, tc.want)
		}
		if m.count != 0 {
			t.Errorf("%v: count %d left pending", tc.keys, m.count)
		}
	}
}

func TestDigitsSwitchSectionsFromSidebar(t *testing.T) {
	m := countModel(5)
	m.focusPane = PaneSidebar
	m, _ = press(t, m, "2")
	if m.sidebar.Section() != SectionQueue {
		t.Error("2 from the sidebar should still open the queue")
	}
}

func TestCountInVisualMode(t *testing.T) {
	m := countModel(10)
	m, _ = press(t, m, "v", "3", "j")
	if lo, hi := m.tracklist.VisualRange(); lo != 0 || hi != 3 {
		t.Errorf("v3j range = %d..%d, want 0..3", lo, hi)
	}
}

func TestCountRespectsReboundDigit(t *testing.T) {
	m := countModel(10)
	keys, err := KeyMapFromOverrides(map[string]string{"next": "5"})
	if err != nil {
		t.Fatal(err)
	}
	m.keys = keys
	m, _ = press(t, m, "5")
	if m.count != 0 {
		t.Error("a digit bound to another action should not start a count")
	}
}
