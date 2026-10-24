//go:build windows

package player

import "os"

// interrupt can't deliver a signal on Windows; Stop falls back to Kill.
func interrupt(p *os.Process) error { return p.Kill() }
