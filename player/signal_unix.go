//go:build !windows

package player

import (
	"os"
	"syscall"
)

// interrupt sends SIGINT, the signal librespot handles to leave Spotify
// Connect cleanly.
func interrupt(p *os.Process) error { return p.Signal(syscall.SIGINT) }
