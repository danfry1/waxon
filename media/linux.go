//go:build linux

package media

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// Linux: waxon registers as an MPRIS player on the session bus
// (org.mpris.MediaPlayer2.waxon), which desktops, media keys and playerctl
// all talk to. Without a session bus (a server, SSH) Start returns
// ErrUnsupported.

const (
	mprisPath   = "/org/mpris/MediaPlayer2"
	mprisRoot   = "org.mpris.MediaPlayer2"
	mprisPlayer = "org.mpris.MediaPlayer2.Player"
	// mprisTrack is the one track id waxon uses: MPRIS wants an object path
	// per track, but it only ever has the playing one.
	mprisTrack = dbus.ObjectPath("/org/waxon/track/current")
)

func runMain(fn func() int) int { return fn() }

type mprisController struct {
	conn  *dbus.Conn
	props *prop.Properties
	mu    sync.Mutex
	pos   time.Duration
}

// mprisPlayerMethods implements org.mpris.MediaPlayer2.Player's methods.
type mprisPlayerMethods struct {
	c       *mprisController
	onEvent func(Event)
}

func (p mprisPlayerMethods) PlayPause() *dbus.Error { p.onEvent(Event{Command: Toggle}); return nil }
func (p mprisPlayerMethods) Play() *dbus.Error      { p.onEvent(Event{Command: Play}); return nil }
func (p mprisPlayerMethods) Pause() *dbus.Error     { p.onEvent(Event{Command: Pause}); return nil }
func (p mprisPlayerMethods) Stop() *dbus.Error      { p.onEvent(Event{Command: Pause}); return nil }
func (p mprisPlayerMethods) Next() *dbus.Error      { p.onEvent(Event{Command: Next}); return nil }
func (p mprisPlayerMethods) Previous() *dbus.Error  { p.onEvent(Event{Command: Previous}); return nil }

// SeekBy is MPRIS's Seek (exported under that name: Go reserves Seek for
// io.Seeker): it moves by offset microseconds from the current position.
func (p mprisPlayerMethods) SeekBy(offset int64) *dbus.Error {
	p.c.mu.Lock()
	to := p.c.pos + time.Duration(offset)*time.Microsecond
	p.c.mu.Unlock()
	p.onEvent(Event{Command: Seek, Position: max(0, to)})
	return nil
}

// SetPosition moves to position microseconds into the track.
func (p mprisPlayerMethods) SetPosition(_ dbus.ObjectPath, position int64) *dbus.Error {
	p.onEvent(Event{Command: Seek, Position: time.Duration(position) * time.Microsecond})
	return nil
}

func (p mprisPlayerMethods) OpenUri(string) *dbus.Error { return nil } //nolint:revive // MPRIS's name

// mprisRootMethods implements org.mpris.MediaPlayer2's methods (no window
// to raise, and quitting is the terminal's business).
type mprisRootMethods struct{}

func (mprisRootMethods) Raise() *dbus.Error { return nil }
func (mprisRootMethods) Quit() *dbus.Error  { return nil }

func start(onEvent func(Event)) (Controller, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	c := &mprisController{conn: conn}
	player := mprisPlayerMethods{c: c, onEvent: onEvent}
	if err := conn.ExportWithMap(player, mprisNames, mprisPath, mprisPlayer); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Export(mprisRootMethods{}, mprisPath, mprisRoot); err != nil {
		conn.Close()
		return nil, err
	}
	ro := func(v any) *prop.Prop { return &prop.Prop{Value: v, Emit: prop.EmitTrue} }
	props, err := prop.Export(conn, mprisPath, prop.Map{
		mprisRoot: {
			"CanQuit":             ro(false),
			"CanRaise":            ro(false),
			"HasTrackList":        ro(false),
			"Identity":            ro("waxon"),
			"SupportedUriSchemes": ro([]string{}),
			"SupportedMimeTypes":  ro([]string{}),
		},
		mprisPlayer: {
			"PlaybackStatus": ro("Stopped"),
			"Rate":           ro(1.0),
			"Metadata":       ro(map[string]dbus.Variant{}),
			"Volume":         ro(1.0),
			"Position":       {Value: int64(0), Emit: prop.EmitFalse}, // read on demand, per the spec
			"MinimumRate":    ro(1.0),
			"MaximumRate":    ro(1.0),
			"CanGoNext":      ro(true),
			"CanGoPrevious":  ro(true),
			"CanPlay":        ro(true),
			"CanPause":       ro(true),
			"CanSeek":        ro(true),
			"CanControl":     ro(true),
		},
	})
	if err != nil {
		conn.Close()
		return nil, err
	}
	c.props = props
	node := &introspect.Node{Name: mprisPath, Interfaces: []introspect.Interface{
		introspect.IntrospectData,
		prop.IntrospectData,
		{Name: mprisRoot, Methods: introspect.Methods(mprisRootMethods{}), Properties: props.Introspection(mprisRoot)},
		{Name: mprisPlayer, Methods: renamed(introspect.Methods(player)), Properties: props.Introspection(mprisPlayer)},
	}}
	if err := conn.Export(introspect.NewIntrospectable(node), mprisPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		conn.Close()
		return nil, err
	}
	// One name per running waxon: the second gets an instance suffix, as
	// MPRIS allows.
	name := "org.mpris.MediaPlayer2.waxon"
	reply, err := conn.RequestName(name, dbus.NameFlagDoNotQueue)
	if err == nil && reply != dbus.RequestNameReplyPrimaryOwner {
		name = fmt.Sprintf("%s.instance%d", name, os.Getpid())
		reply, err = conn.RequestName(name, dbus.NameFlagDoNotQueue)
	}
	if err != nil || reply != dbus.RequestNameReplyPrimaryOwner {
		conn.Close()
		return nil, fmt.Errorf("%w: couldn't take an MPRIS name", ErrUnsupported)
	}
	return c, nil
}

func (c *mprisController) Update(s State) {
	c.mu.Lock()
	c.pos = s.Position
	c.mu.Unlock()
	status := "Paused"
	switch {
	case s.Empty:
		status = "Stopped"
	case s.Playing:
		status = "Playing"
	}
	c.props.SetMust(mprisPlayer, "Position", s.Position.Microseconds())
	c.props.SetMust(mprisPlayer, "Metadata", mprisMetadata(s))
	c.props.SetMust(mprisPlayer, "PlaybackStatus", status)
}

// mprisMetadata is s as MPRIS track metadata.
func mprisMetadata(s State) map[string]dbus.Variant {
	if s.Empty {
		return map[string]dbus.Variant{}
	}
	md := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(mprisTrack),
		"mpris:length":  dbus.MakeVariant(s.Duration.Microseconds()),
		"xesam:title":   dbus.MakeVariant(s.Title),
		"xesam:album":   dbus.MakeVariant(s.Album),
		"xesam:artist":  dbus.MakeVariant([]string{s.Artist}),
	}
	if s.ArtworkURL != "" {
		md["mpris:artUrl"] = dbus.MakeVariant(s.ArtworkURL)
	}
	return md
}

func (c *mprisController) Close() { _ = c.conn.Close() }

// mprisNames maps Go method names to MPRIS's where they differ.
var mprisNames = map[string]string{"SeekBy": "Seek"}

// renamed applies mprisNames to introspected methods.
func renamed(ms []introspect.Method) []introspect.Method {
	for i := range ms {
		if n, ok := mprisNames[ms[i].Name]; ok {
			ms[i].Name = n
		}
	}
	return ms
}
