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
)

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
			return Command{}, errors.New("usage: vol <0-100>")
		}
		v, err := strconv.Atoi(args[0])
		if err != nil || v < 0 || v > 100 {
			return Command{}, errors.New("volume must be 0-100")
		}
		return Command{Type: CmdVolume, IntArg: v}, nil

	case "shuffle":
		// Toggle — no arguments needed
		return Command{Type: CmdShuffle}, nil

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
		return Command{Type: CmdDevice}, nil

	case "search":
		query := strings.Join(args, " ")
		return Command{Type: CmdSearch, StrArg: query}, nil

	case "recent":
		return Command{Type: CmdRecent}, nil

	default:
		return Command{}, fmt.Errorf("unknown command: %s", cmd)
	}
}
