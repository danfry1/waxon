package app

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/danfry1/waxon/source"
)

// barColumns returns the first column and width of the progress bar on the
// given screen row of the rendered view, found by its glyphs.
func barColumns(t *testing.T, view string, row int) (int, int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if row >= len(lines) {
		t.Fatalf("row %d beyond view (%d lines)", row, len(lines))
	}
	runes := []rune(ansi.Strip(lines[row]))
	start, end := -1, -1
	for i, r := range runes {
		if r == '━' || r == '●' || r == '─' {
			if start < 0 {
				start = i
			}
			end = i
		} else if start >= 0 && r == ' ' && end == i-1 && i+1 < len(runes) && runes[i+1] != '─' {
			break
		}
	}
	if start < 0 {
		t.Fatalf("no progress bar on row %d: %q", row, string(runes))
	}
	return start, end - start + 1
}

func TestProgressBarSpanMatchesRender(t *testing.T) {
	for _, tc := range []struct {
		name          string
		width, height int
	}{
		{"compact", 100, 20},
		{"art", 120, 40},
		{"art-long-names", 80, 40},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestModel(&StubSource{})
			m.width, m.height = tc.width, tc.height
			m.layoutResize()
			m.track = &source.Track{ID: "t", Name: "Song", Artist: "Artist", Album: "Album", Duration: 3 * time.Minute, Position: time.Minute}
			if strings.HasSuffix(tc.name, "long-names") {
				m.track.Name = strings.Repeat("Very Long Title ", 6)
				m.track.Album = strings.Repeat("Deluxe Anniversary Edition ", 4)
			}
			row, x0, w, ok := m.statusbar.ProgressBarSpan(m.track, m.shuffleOn, m.repeatMode, m.liked, m.artStatusBar())
			if !ok {
				t.Fatal("expected a progress bar")
			}
			screenRow := m.height - m.statusRows() + row
			gotX, gotW := barColumns(t, m.View(), screenRow)
			if gotX != x0 || gotW != w {
				t.Errorf("span x=%d w=%d, rendered x=%d w=%d", x0, w, gotX, gotW)
			}
		})
	}
}

func TestClickProgressBarSeeks(t *testing.T) {
	var seeked time.Duration = -1
	stub := &StubSource{SeekFn: func(_ context.Context, pos time.Duration) error { seeked = pos; return nil }}
	m := newTestModel(stub)
	m.track = &source.Track{ID: "t", Name: "Song", Duration: 100 * time.Second, Position: 10 * time.Second, Playing: true}
	row, x0, w, _ := m.statusbar.ProgressBarSpan(m.track, false, "", false, m.artStatusBar())
	y := m.height - m.statusRows() + row

	result, cmd := m.Update(tea.MouseMsg{X: x0 + w - 1, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	runCmdTree(cmd)
	if seeked != 100*time.Second {
		t.Errorf("click at the bar's end should seek to the end; got %v", seeked)
	}
	if got := result.(Model).track.Position; got != 100*time.Second {
		t.Errorf("bar should move immediately; position %v", got)
	}

	seeked = -1
	_, cmd = m.Update(tea.MouseMsg{X: x0 - 1, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	runCmdTree(cmd)
	if seeked != -1 {
		t.Error("a click left of the bar should not seek")
	}
}
