package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/app"
	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/config"
	"github.com/danfry1/waxon/player"
	"github.com/danfry1/waxon/source"
)

// runPlayer handles `waxon player login|off`.
func runPlayer(args []string) int {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	cfg, _ := config.Load()
	switch sub {
	case "login":
		bin, err := player.Find(cfg.PlayerPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		paste := myauth.IsRemoteSession()
		fmt.Println("Signing waxon's player in to Spotify. librespot needs its own login:")
		fmt.Println("Spotify only lets its own apps play audio, so waxon's login can't be reused.")
		if paste {
			fmt.Println("Open the address below in any browser, approve, then paste the address it lands on here.")
		} else {
			fmt.Println("Approve in the browser window that opens (or open the address below).")
		}
		fmt.Println()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		if err := player.Login(ctx, bin, player.Dir(), paste, os.Stdin, os.Stdout); err != nil {
			fmt.Fprintf(os.Stderr, "\nPlayer sign-in failed: %v\n", err)
			return 1
		}
		if err := config.Save(config.Config{Player: player.Backend}); err != nil {
			fmt.Fprintf(os.Stderr, "Signed in, but could not turn the player on in config.json: %v\n", err)
			return 1
		}
		fmt.Printf("\nDone. waxon now plays through its own device, %q, whenever nothing else is playing.\n", player.DeviceName())
		fmt.Println("Turn it off with 'waxon player off'.")
		return 0
	case "off":
		if err := config.Save(config.Config{Player: "off"}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		fmt.Println("waxon's player is off; waxon controls your other Spotify devices only. ('waxon player login' turns it back on.)")
		return 0
	default:
		fmt.Fprintln(os.Stderr, `Usage:
  waxon player login   Sign in librespot and play through waxon itself
  waxon player off     Stop starting the player`)
		return 2
	}
}

// playerRunner owns the librespot sidecar for one TUI session.
type playerRunner struct {
	bin, dir, name string

	mu      sync.Mutex
	sidecar *player.Sidecar
	stopped bool
}

// newPlayerRunner returns a runner when the player is turned on in cfg,
// warning (and returning nil) when it can't run.
func newPlayerRunner(cfg config.Config) *playerRunner {
	if cfg.Player != player.Backend {
		return nil
	}
	bin, err := player.Find(cfg.PlayerPath)
	if err == nil && !player.LoggedIn(player.Dir()) {
		err = player.ErrNoLogin
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: waxon's player: %v\n", err)
		time.Sleep(1500 * time.Millisecond) // the TUI is about to take over the screen
		return nil
	}
	player.Secure(player.Dir())
	return &playerRunner{bin: bin, dir: player.Dir(), name: player.DeviceName()}
}

// start launches librespot in the background unless this machine's waxon
// device is already online (another waxon is running it), reporting an
// unexpected exit to the TUI.
func (r *playerRunner) start(ctx context.Context, src source.RichSource, p *tea.Program) {
	if r == nil {
		return
	}
	go func() {
		lookup, cancel := context.WithTimeout(ctx, 3*time.Second)
		devices, err := src.Devices(lookup)
		cancel()
		if err == nil {
			for _, d := range devices {
				if d.Name == r.name {
					slog.Info("player already running elsewhere", "device", r.name)
					return
				}
			}
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		if r.stopped {
			return
		}
		s, err := player.Start(r.bin, r.dir, r.name, playerLog(), func(err error) {
			slog.Warn("player exited", "error", err)
			p.Send(app.PlayerExitedMsg{Err: err})
		})
		if err != nil {
			p.Send(app.PlayerExitedMsg{Err: err})
			return
		}
		r.sidecar = s
	}()
}

func (r *playerRunner) stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.stopped = true
	s := r.sidecar
	r.mu.Unlock()
	s.Stop()
}

// playerLog is where librespot's own log goes: the WAXON_LOG file when set.
func playerLog() io.Writer {
	if path := os.Getenv("WAXON_LOG"); path != "" {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
			return f
		}
	}
	return io.Discard
}
