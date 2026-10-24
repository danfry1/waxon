package app

import (
	"testing"

	"github.com/danfry1/waxon/source"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		input string
		want  Command
	}{
		{"q", Command{Type: CmdQuit}},
		{"quit", Command{Type: CmdQuit}},
		{"vol 75", Command{Type: CmdVolume, IntArg: 75}},
		{"volume 0", Command{Type: CmdVolume, IntArg: 0}},
		{"volume 100", Command{Type: CmdVolume, IntArg: 100}},
		{"shuffle", Command{Type: CmdShuffle}},
		{"repeat off", Command{Type: CmdRepeat, StrArg: "off"}},
		{"repeat all", Command{Type: CmdRepeat, StrArg: "context"}},
		{"repeat context", Command{Type: CmdRepeat, StrArg: "context"}},
		{"repeat one", Command{Type: CmdRepeat, StrArg: "track"}},
		{"repeat track", Command{Type: CmdRepeat, StrArg: "track"}},
		{"device", Command{Type: CmdDevice}},
		{"devices", Command{Type: CmdDevice}},
		{"search", Command{Type: CmdSearch}},
		{"search olivia dean", Command{Type: CmdSearch, StrArg: "olivia dean"}},
		{"recent", Command{Type: CmdRecent}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseCommand(tt.input)
			if err != nil {
				t.Fatalf("ParseCommand(%q) error: %v", tt.input, err)
			}
			if got.Type != tt.want.Type {
				t.Errorf("Type = %v, want %v", got.Type, tt.want.Type)
			}
			if got.IntArg != tt.want.IntArg {
				t.Errorf("IntArg = %d, want %d", got.IntArg, tt.want.IntArg)
			}
			if got.StrArg != tt.want.StrArg {
				t.Errorf("StrArg = %q, want %q", got.StrArg, tt.want.StrArg)
			}
		})
	}
}

func TestParseCommandErrors(t *testing.T) {
	bad := []string{
		"notacommand",
		"vol 150",
		"vol -1",
		"vol abc",
		"vol",
		"repeat",
		"repeat invalid",
		"",
	}
	for _, input := range bad {
		t.Run(input, func(t *testing.T) {
			_, err := ParseCommand(input)
			if err == nil {
				t.Fatalf("ParseCommand(%q) expected error", input)
			}
		})
	}
}

func TestNowPlayingCommandsOpenView(t *testing.T) {
	for _, tc := range []struct {
		input         string
		lyrics, vinyl bool
	}{
		{"lyrics", true, false},
		{"vinyl", false, true},
	} {
		m := newTestModel(&StubSource{})
		m.track = &source.Track{ID: "t1", Name: "Song"}
		m.lyricsView, m.vinylMode = !tc.lyrics, !tc.vinyl // the other view was last open
		m.executeCommand(tc.input)
		if m.mode != ModeNowPlaying || m.lyricsView != tc.lyrics || m.vinylMode != tc.vinyl {
			t.Errorf(":%s → mode=%v lyrics=%v vinyl=%v", tc.input, m.mode, m.lyricsView, m.vinylMode)
		}
	}
}

func TestNowPlayingReopensAsLeft(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.track = &source.Track{ID: "t1", Name: "Song"}
	for _, k := range []string{"N", "l", "N", "N"} { // open, lyrics, close, reopen
		res, _ := m.Update(runeKey(k))
		m = res.(Model)
	}
	if m.mode != ModeNowPlaying || !m.lyricsView {
		t.Errorf("reopening Now Playing should show the lyrics again; mode=%v lyrics=%v", m.mode, m.lyricsView)
	}
}
