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

func TestCLIStyleCommands(t *testing.T) {
	for in, want := range map[string]Command{
		"next":       {Type: CmdControl, StrArg: "next"},
		"previous":   {Type: CmdControl, StrArg: "prev"},
		"pause":      {Type: CmdControl, StrArg: "pause"},
		"seek +30":   {Type: CmdSeek, IntArg: 30, StrArg: "+"},
		"seek 1:30":  {Type: CmdSeek, IntArg: 90},
		"vol -10":    {Type: CmdVolume, IntArg: -10, StrArg: "rel"},
		"vol 40":     {Type: CmdVolume, IntArg: 40},
		"shuffle on": {Type: CmdShuffle, StrArg: "on"},
		"shuffle":    {Type: CmdShuffle},
		"like off":   {Type: CmdLike, StrArg: "off"},
		"mute":       {Type: CmdMute},
		"h":          {Type: CmdHelp},
	} {
		got, err := ParseCommand(in)
		if err != nil || got != want {
			t.Errorf("%q = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, bad := range []string{"seek", "seek x", "shuffle maybe", "vol loud"} {
		if _, err := ParseCommand(bad); err == nil {
			t.Errorf("%q should be an error", bad)
		}
	}
}

func TestCompleteCommand(t *testing.T) {
	for in, want := range map[string]string{
		"ne":    "next ",
		"p":     "p",        // play, playlist new, podcasts, prev, pause: no common part past p
		"pla":   "play",     // play and playlist new
		"sh":    "shuffle ", // the only one
		"vol 5": "vol 5",    // an argument: left alone
		"zzz":   "zzz",
	} {
		if got := completeCommand(in); got != want {
			t.Errorf("complete(%q) = %q, want %q", in, got, want)
		}
	}
}

// :vol -10 lowers the volume by 10 from where it is.
func TestRelativeVolumeCommand(t *testing.T) {
	m := newTestModel(&StubSource{})
	m.volume = 50
	m.runCommand(Command{Type: CmdVolume, IntArg: -10, StrArg: "rel"})
	if m.volume != 40 {
		t.Errorf("volume = %d, want 40", m.volume)
	}
}
