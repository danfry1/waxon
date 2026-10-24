package app

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type CmdType int

const (
	CmdQuit CmdType = iota
	CmdVolume
	CmdShuffle
	CmdRepeat
	CmdDevice
	CmdSearch
	CmdRecent
	CmdCollection // StrArg: collection ID (saved albums, followed artists, podcasts)
	CmdTheme
	CmdNewPlaylist
	CmdNowPlaying // IntArg: the npView to open on
	CmdControl    // StrArg: play, pause, toggle, next or prev
	CmdSeek       // IntArg: seconds; StrArg: "+" / "-" for relative, "" for absolute
	CmdLike       // StrArg: "on", "off" or "" (toggle), for the playing track
	CmdMute
	CmdHelp
)

// commandHelp is the user-facing list of : commands shown in the help overlay.
// Keep in sync with ParseCommand.
var commandHelp = []helpBinding{
	{":play / :pause", "resume / pause"},
	{":next / :prev", "next / previous track"},
	{":seek ±N / M:SS", "seek by N s, or to a time"},
	{":vol N / ±N", "set or change volume"},
	{":mute", "mute / unmute"},
	{":like [on|off]", "like the playing track"},
	{":shuffle [on|off]", "shuffle"},
	{":repeat X", "off / all / one"},
	{":device", "switch device"},
	{":device -", "back to the previous device"},
	{":device NAME", "switch to the device named NAME"},
	{":search Q", "search Spotify"},
	{":recent", "recently played"},
	{":albums", "saved albums"},
	{":artists", "followed artists"},
	{":podcasts", "podcasts"},
	{":lyrics", "now playing: lyrics"},
	{":vinyl", "now playing: vinyl"},
	{":theme NAME", "switch colour theme"},
	{":playlist new NAME", "create a playlist"},
	{":h", "help"},
	{":q", "quit"},
}

// commandNames are the commands Tab completes, ParseCommand's spellings.
var commandNames = []string{
	"albums", "artists", "device", "help", "like", "lyrics", "mute", "next", "pause",
	"play", "playlist new", "podcasts", "prev", "quit", "recent", "repeat", "search",
	"seek", "shuffle", "theme", "toggle", "vinyl", "vol",
}

// completeCommand completes the command name being typed in input to the
// longest prefix the matching commands share; input comes back unchanged
// if nothing matches or an argument is already being typed.
func completeCommand(input string) string {
	if strings.Contains(strings.TrimLeft(input, " "), " ") {
		return input
	}
	prefix := strings.TrimSpace(input)
	var match []string
	for _, n := range commandNames {
		if strings.HasPrefix(n, prefix) {
			match = append(match, n)
		}
	}
	if len(match) == 0 {
		return input
	}
	common := match[0]
	for _, n := range match[1:] {
		for !strings.HasPrefix(n, common) {
			common = common[:len(common)-1]
		}
	}
	if len(match) == 1 {
		common += " "
	}
	return common
}

type Command struct {
	Type   CmdType
	IntArg int
	StrArg string
}

func ParseCommand(input string) (Command, error) {
	input = strings.TrimSpace(input)
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return Command{}, errors.New("empty command")
	}

	cmd := parts[0]
	args := parts[1:]

	switch cmd {
	case "q", "quit":
		return Command{Type: CmdQuit}, nil

	case "vol", "volume":
		if len(args) != 1 {
			return Command{}, errors.New("usage: vol <0-100|+N|-N>")
		}
		arg := args[0]
		v, err := strconv.Atoi(strings.TrimPrefix(arg, "+"))
		if err != nil {
			return Command{}, errors.New("usage: vol <0-100|+N|-N>")
		}
		if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") {
			return Command{Type: CmdVolume, IntArg: v, StrArg: "rel"}, nil
		}
		if v < 0 || v > 100 {
			return Command{}, errors.New("volume must be 0-100")
		}
		return Command{Type: CmdVolume, IntArg: v}, nil

	case "shuffle":
		on, err := onOffArg(args, "shuffle [on|off]")
		return Command{Type: CmdShuffle, StrArg: on}, err

	case "play", "pause", "toggle", "next", "prev", "previous":
		if cmd == "previous" {
			cmd = "prev"
		}
		return Command{Type: CmdControl, StrArg: cmd}, nil

	case "seek":
		if len(args) != 1 {
			return Command{}, errors.New("usage: seek <+N|-N|SECONDS|M:SS>")
		}
		sign := ""
		if a := args[0]; strings.HasPrefix(a, "+") || strings.HasPrefix(a, "-") {
			sign = a[:1]
		}
		secs, err := parseSeconds(strings.TrimLeft(args[0], "+-"))
		if err != nil {
			return Command{}, errors.New("usage: seek <+N|-N|SECONDS|M:SS>")
		}
		return Command{Type: CmdSeek, IntArg: secs, StrArg: sign}, nil

	case "like":
		on, err := onOffArg(args, "like [on|off]")
		return Command{Type: CmdLike, StrArg: on}, err

	case "mute":
		return Command{Type: CmdMute}, nil

	case "h", "help":
		return Command{Type: CmdHelp}, nil

	case "repeat":
		if len(args) != 1 {
			return Command{}, errors.New("usage: repeat off|all|one")
		}
		mode := args[0]
		switch mode {
		case "off":
			// keep as "off"
		case "all", "context":
			mode = "context"
		case "one", "track":
			mode = "track"
		default:
			return Command{}, errors.New("repeat mode must be off|all|one")
		}
		return Command{Type: CmdRepeat, StrArg: mode}, nil

	case "device", "devices":
		return Command{Type: CmdDevice, StrArg: strings.Join(args, " ")}, nil

	case "search":
		query := strings.Join(args, " ")
		return Command{Type: CmdSearch, StrArg: query}, nil

	case "recent":
		return Command{Type: CmdRecent}, nil

	case "albums":
		return Command{Type: CmdCollection, StrArg: collectionAlbumsID}, nil
	case "artists":
		return Command{Type: CmdCollection, StrArg: collectionArtistsID}, nil
	case "podcasts", "shows":
		return Command{Type: CmdCollection, StrArg: collectionPodcastsID}, nil

	case "lyrics":
		return Command{Type: CmdNowPlaying, IntArg: int(npViewLyrics)}, nil
	case "vinyl":
		return Command{Type: CmdNowPlaying, IntArg: int(npViewVinyl)}, nil

	case "playlist", "pl":
		if len(args) >= 2 && args[0] == "new" {
			return Command{Type: CmdNewPlaylist, StrArg: strings.Join(args[1:], " ")}, nil
		}
		return Command{}, errors.New("usage: playlist new <name>")

	case "theme", "themes", "colorscheme", "colourscheme":
		if len(args) != 1 {
			return Command{}, fmt.Errorf("usage: theme <%s>", strings.Join(ThemeNames(), "|"))
		}
		return Command{Type: CmdTheme, StrArg: args[0]}, nil

	default:
		return Command{}, fmt.Errorf("unknown command: %s", cmd)
	}
}

// onOffArg reads an optional on/off argument ("" when absent: toggle).
func onOffArg(args []string, usage string) (string, error) {
	switch {
	case len(args) == 0:
		return "", nil
	case len(args) == 1 && (args[0] == "on" || args[0] == "off"):
		return args[0], nil
	}
	return "", errors.New("usage: " + usage)
}

// parseSeconds reads SECONDS, M:SS or H:MM:SS.
func parseSeconds(s string) (int, error) {
	parts := strings.Split(s, ":")
	if len(parts) > 3 {
		return 0, errors.New("bad time")
	}
	total := 0
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return 0, errors.New("bad time")
		}
		total = total*60 + n
	}
	return total, nil
}
