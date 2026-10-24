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
		if interactiveTerminal() {
			// PlayerOn, not Finished: leaving the last screen with ctrl+c
			// still leaves the player signed in and on.
			if !runSetup(setupDeps("", ""), app.SetupPlayerOnly).PlayerOn() {
				return 1
			}
			fmt.Printf("waxon now plays through its own device, %q, whenever nothing else is playing.\n", player.DeviceName())
			return 0
		}
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
	case "", "status":
		printPlayerStatus(cfg)
		return 0
	default:
		fmt.Fprintln(os.Stderr, playerUsage)
		return 2
	}
}

const playerUsage = `Usage:
  waxon player         Show the player's status
  waxon player login   Sign librespot in and play through waxon itself
  waxon player off     Stop starting the player`

// printPlayerStatus says whether waxon's own player is set up and, if not,
// the next step.
func printPlayerStatus(cfg config.Config) {
	bin, findErr := player.Find(cfg.PlayerPath)
	loggedIn := player.LoggedIn(player.Dir())
	on := cfg.Player == player.Backend

	fmt.Println("waxon's player plays Spotify right here, through librespot, so no Spotify")
	fmt.Println("app has to be open. It's optional: without it waxon controls your other")
	fmt.Println("Spotify devices (phone, desktop app, speakers).")
	fmt.Println()
	mark := func(ok bool) string {
		if ok {
			return "✓"
		}
		return "✗"
	}
	if findErr == nil {
		fmt.Printf("  %s librespot   %s\n", mark(true), bin)
	} else {
		fmt.Printf("  %s librespot   not installed\n", mark(false))
	}
	if loggedIn {
		fmt.Printf("  %s signed in\n", mark(true))
	} else {
		fmt.Printf("  %s not signed in\n", mark(false))
	}
	if on {
		fmt.Printf("  %s on          plays as %q\n", mark(true), player.DeviceName())
	} else {
		fmt.Printf("  %s off\n", mark(false))
	}
	fmt.Println()
	switch {
	case findErr != nil:
		fmt.Printf("Next: install librespot with '%s', then run 'waxon player login'.\n", player.InstallCommand())
	case !loggedIn || !on:
		fmt.Println("Next: run 'waxon player login' to sign in and turn it on.")
	default:
		fmt.Println("All set. 'waxon player off' turns it off.")
	}
}

// playerRunner owns the librespot sidecar for one TUI session.
type playerRunner struct {
	bin, dir, name string

	mu      sync.Mutex
	sidecar *player.Sidecar
	stopped bool
}

// newPlayerRunner returns a runner when the player is turned on in cfg. When
// it's on but can't run, it returns nil and a short problem-and-fix for the
// TUI to show.
func newPlayerRunner(cfg config.Config) (*playerRunner, string) {
	if cfg.Player != player.Backend {
		return nil, ""
	}
	bin, err := player.Find(cfg.PlayerPath)
	if err != nil {
		return nil, "librespot isn't installed: " + player.InstallCommand()
	}
	if !player.LoggedIn(player.Dir()) {
		return nil, "Not signed in: run 'waxon player login'"
	}
	player.Secure(player.Dir())
	return &playerRunner{bin: bin, dir: player.Dir(), name: player.DeviceName()}, ""
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
