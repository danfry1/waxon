package app

import "testing"

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
