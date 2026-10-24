// Package media connects waxon to the operating system's media controls:
// the keyboard's play/pause, next and previous keys, and the "Now Playing"
// widgets (Control Center on macOS, MPRIS on Linux desktops). waxon tells it
// what's playing; it reports the buttons pressed. waxon handles those like
// its own keys, so they control whatever waxon controls — its own player,
// a phone, a speaker.
//
// Backends: macOS (MediaPlayer framework, via purego, no cgo) and Linux
// (MPRIS over D-Bus). Elsewhere Start returns ErrUnsupported.
package media

import (
	"errors"
	"image"
	"time"
)

// State is what's playing, for the system's Now Playing display.
type State struct {
	// ID identifies the track (a Spotify id, or URI for a local file).
	ID                   string
	Title, Artist, Album string
	Duration, Position   time.Duration
	Playing              bool
	// ArtworkURL identifies Artwork: the backend only re-sends the image
	// when it changes.
	ArtworkURL string
	Artwork    image.Image
	// Empty means nothing is playing; the backend clears its display.
	Empty bool
}

// Command is a media control pressed.
type Command int

const (
	Toggle Command = iota // play/pause
	Play
	Pause
	Next
	Previous
	Seek // to Event.Position
)

// Event is a press of a media control.
type Event struct {
	Command  Command
	Position time.Duration // for Seek
}

// Controller publishes the playing state to the system.
type Controller interface {
	Update(State)
	Close()
}

// ErrUnsupported is returned by Start where there are no media controls to
// connect to (an OS without a backend, or no desktop session).
var ErrUnsupported = errors.New("no system media controls here")

// Start connects to the system's media controls, calling onEvent (from any
// goroutine) for each press.
func Start(onEvent func(Event)) (Controller, error) { return start(onEvent) }

// RunMain runs fn, the program's main work, and returns its result. On
// macOS the system delivers media events to an event loop on the main
// thread, so RunMain runs that loop there and fn on another goroutine; it
// must be called from main (the package locks main to the main thread).
// Elsewhere it simply calls fn.
func RunMain(fn func() int) int { return runMain(fn) }
