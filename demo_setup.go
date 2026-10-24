//go:build demo

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/danfry1/waxon/app"
	"github.com/danfry1/waxon/source"
)

// demoSetupDeps drive the guided setup with stand-ins for the browser
// sign-in, the device list and librespot, so `waxon demo setup` shows the
// whole flow without a Spotify account (and records reproducibly).
func demoSetupDeps() (deps app.SetupDeps, cleanup func()) {
	p := &demoPlayer{}
	return app.SetupDeps{
		SharedID:     "d420a117a32841c2b3474932e49fb54b",
		RedirectURI:  func(string) string { return "http://127.0.0.1:8888/callback" },
		DashboardURL: DashboardURL,
		BeginLogin:   func(string) (app.SetupLogin, error) { return demoLogin{}, nil },
		Devices: func(context.Context) ([]source.Device, error) {
			return []source.Device{{Name: "iPhone"}, {Name: "Living Room"}}, nil
		},
		Player:      p,
		OpenBrowser: func(string) error { return nil },
		Copy:        func(string) error { return nil },
	}, func() { _ = os.Remove(p.markerPath()) }
}

// demoLogin is approved in the browser a moment after it starts.
type demoLogin struct{}

func (demoLogin) URL() string        { return "https://accounts.spotify.com/authorize?client_id=…" }
func (demoLogin) Listening() bool    { return true }
func (demoLogin) Paste(string) error { return nil }
func (demoLogin) Close()             {}
func (demoLogin) Wait(ctx context.Context) error {
	select {
	case <-time.After(2500 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// demoPlayer starts uninstalled; its "install" prints like a package manager
// and leaves a marker file that counts as installed.
type demoPlayer struct {
	once   sync.Once
	marker string
}

func (p *demoPlayer) markerPath() string {
	p.once.Do(func() {
		p.marker = filepath.Join(os.TempDir(), fmt.Sprintf("waxon-demo-librespot-%d", os.Getpid()))
	})
	return p.marker
}

func (p *demoPlayer) Installed() bool {
	_, err := os.Stat(p.markerPath())
	return err == nil
}

func (p *demoPlayer) LoggedIn() bool { return false }

func (p *demoPlayer) Installer() ([]string, string, string) {
	script := `echo "==> Fetching librespot"; sleep 0.6; echo "==> Pouring librespot--0.8.0.bottle.tar.gz"; sleep 0.6; echo "🍺  librespot was installed"; touch "$1"; sleep 0.8`
	return []string{"sh", "-c", script, "sh", p.markerPath()}, "Homebrew", "brew install librespot"
}

func (p *demoPlayer) Login(ctx context.Context, _ bool, _ io.Reader, out io.Writer) error {
	fmt.Fprintln(out, "Browse to: https://accounts.spotify.com/authorize?client_id=…")
	select {
	case <-time.After(2500 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *demoPlayer) Enable() error      { return nil }
func (p *demoPlayer) DeviceName() string { return "waxon (Mac)" }
