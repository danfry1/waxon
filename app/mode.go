package app

// Mode represents the current input mode.
type Mode int

const (
	ModeNormal Mode = iota
	ModeCommand
	ModeSearch
	ModeFilter
	ModeHelp
	ModeActions
	ModeDevices
	ModeNowPlaying
	ModePlaylistPick
	ModeVisual
	ModeSetup
	ModeSettings
)

func (m Mode) String() string {
	switch m {
	case ModeNormal:
		return "NORMAL"
	case ModeCommand:
		return "COMMAND"
	case ModeSearch:
		return "SEARCH"
	case ModeFilter:
		return "FILTER"
	case ModeHelp:
		return "HELP"
	case ModeActions:
		return "ACTIONS"
	case ModeDevices:
		return "DEVICES"
	case ModeNowPlaying:
		return "NOW PLAYING"
	case ModePlaylistPick:
		return "PLAYLIST"
	case ModeVisual:
		return "VISUAL"
	case ModeSetup:
		return "SETUP"
	case ModeSettings:
		return "SETTINGS"
	default:
		return ""
	}
}

// Pane identifies which pane has focus.
type Pane int

const (
	PaneSidebar Pane = iota
	PaneTrackList
)

func (p Pane) String() string {
	switch p {
	case PaneSidebar:
		return "SIDEBAR"
	case PaneTrackList:
		return "TRACKLIST"
	default:
		return ""
	}
}
