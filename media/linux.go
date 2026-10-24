//go:build linux

package media

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"
)

// Linux: waxon registers as an MPRIS player on the session bus
// (org.mpris.MediaPlayer2.waxon), which desktops, media keys and playerctl
// all talk to. Without a running session bus (a server, SSH) Start returns
// ErrUnsupported; it never launches one.

const (
	mprisPath   = "/org/mpris/MediaPlayer2"
	mprisRoot   = "org.mpris.MediaPlayer2"
	mprisPlayer = "org.mpris.MediaPlayer2.Player"
	// mprisNoTrack is MPRIS's "no track" id.
	mprisNoTrack = dbus.ObjectPath("/org/mpris/MediaPlayer2/TrackList/NoTrack")
	// mprisTick is how often the Position property is brought up to date
	// while playing (it's local: no bus traffic unless a client reads it).
	mprisTick = time.Second
)

func runMain(fn func() int) int { return fn() }

type mprisController struct {
	conn  *dbus.Conn
	props *prop.Properties
	done  chan struct{}
	once  sync.Once

	mu      sync.Mutex
	pos     time.Duration // at at
	at      time.Time
	playing bool
	track   dbus.ObjectPath
	dead    bool // the bus went away; stop talking to it
}

// position is where the track is now, extrapolated from the last update.
func (c *mprisController) position() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.positionLocked(time.Now())
}

func (c *mprisController) positionLocked(now time.Time) time.Duration {
	if c.playing {
		return c.pos + now.Sub(c.at)
	}
	return c.pos
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
	to := p.c.position() + time.Duration(offset)*time.Microsecond
	p.onEvent(Event{Command: Seek, Position: max(0, to)})
	return nil
}

// SetPosition moves to position microseconds into track, if that's still
// the one playing (as the spec asks).
func (p mprisPlayerMethods) SetPosition(track dbus.ObjectPath, position int64) *dbus.Error {
	p.c.mu.Lock()
	current := p.c.track
	p.c.mu.Unlock()
	if track != current || position < 0 {
		return nil
	}
	p.onEvent(Event{Command: Seek, Position: time.Duration(position) * time.Microsecond})
	return nil
}

func (p mprisPlayerMethods) OpenUri(string) *dbus.Error { return nil } //nolint:revive // MPRIS's name

// mprisRootMethods implements org.mpris.MediaPlayer2's methods (no window
// to raise, and quitting is the terminal's business).
type mprisRootMethods struct{}

func (mprisRootMethods) Raise() *dbus.Error { return nil }
func (mprisRootMethods) Quit() *dbus.Error  { return nil }

// connect joins the running session bus, never starting one (godbus would
// otherwise run dbus-launch, leaving a daemon behind).
func connect() (*dbus.Conn, error) {
	conn, err := dbus.SessionBusPrivateNoAutoStartup()
	if err != nil {
		return nil, err
	}
	if err := conn.Auth(nil); err != nil {
		conn.Close()
		return nil, err
	}
	if err := conn.Hello(); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

func start(onEvent func(Event)) (Controller, error) {
	conn, err := connect()
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}
	c := &mprisController{conn: conn, done: make(chan struct{}), track: mprisNoTrack}
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
	go c.tick()
	return c, nil
}

// tick keeps the Position property current while playing, for clients
// that read it.
func (c *mprisController) tick() {
	t := time.NewTicker(mprisTick)
	defer t.Stop()
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			c.mu.Lock()
			playing, pos := c.playing, c.positionLocked(time.Now())
			c.mu.Unlock()
			if playing {
				c.set("Position", pos.Microseconds())
			}
		}
	}
}

// set sets a Player property, emitting the change where it's flagged to.
// godbus panics if the emit fails (the bus went away); that ends media
// controls, not waxon.
func (c *mprisController) set(name string, v any) {
	c.mu.Lock()
	dead := c.dead
	c.mu.Unlock()
	if dead {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			c.mu.Lock()
			c.dead = true
			c.mu.Unlock()
			slog.Warn("media controls stopped: lost the session bus", "error", r)
		}
	}()
	c.props.SetMust(mprisPlayer, name, v)
}

func (c *mprisController) Update(s State) {
	now := time.Now()
	track := mprisTrackID(s)
	c.mu.Lock()
	// A position that isn't where playback would have got to by itself is a
	// seek, which clients that extrapolate need telling about.
	jumped := c.track == track && absDuration(s.Position-c.positionLocked(now)) > time.Second
	c.pos, c.at, c.playing, c.track = s.Position, now, s.Playing && !s.Empty, track
	c.mu.Unlock()
	status := "Paused"
	switch {
	case s.Empty:
		status = "Stopped"
	case s.Playing:
		status = "Playing"
	}
	c.set("Position", s.Position.Microseconds())
	c.set("Metadata", mprisMetadata(s))
	c.set("PlaybackStatus", status)
	if jumped {
		c.emitSeeked(s.Position)
	}
}

func (c *mprisController) emitSeeked(pos time.Duration) {
	c.mu.Lock()
	dead := c.dead
	c.mu.Unlock()
	if !dead {
		_ = c.conn.Emit(mprisPath, mprisPlayer+".Seeked", pos.Microseconds())
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

// mprisTrackID is the MPRIS track id for s: an object path made from its
// id, unique per track as the spec asks.
func mprisTrackID(s State) dbus.ObjectPath {
	if s.Empty || s.ID == "" {
		return mprisNoTrack
	}
	var b strings.Builder
	for _, r := range s.ID {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_') // object path elements are [A-Za-z0-9_]
		}
	}
	return dbus.ObjectPath("/org/waxon/track/" + b.String())
}

// mprisMetadata is s as MPRIS track metadata.
func mprisMetadata(s State) map[string]dbus.Variant {
	if s.Empty {
		return map[string]dbus.Variant{}
	}
	md := map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(mprisTrackID(s)),
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

func (c *mprisController) Close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}

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
