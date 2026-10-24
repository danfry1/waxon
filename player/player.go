// Package player runs librespot as a local Spotify Connect device, so waxon
// can play without another Spotify client open.
//
// librespot has to sign in with its own OAuth login: Spotify lets only its
// privileged client IDs drive Connect, so waxon's Web API token is refused.
// Login runs that sign-in once; its credentials are cached in Dir, and Start
// then logs in from the cache without a browser.
package player

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/danfry1/waxon/config"
)

// Backend is the config value that turns the player on.
const Backend = "librespot"

// ErrNotInstalled means the librespot binary couldn't be found.
var ErrNotInstalled = errors.New("librespot not found: install it (e.g. 'brew install librespot' or 'cargo install librespot'), or set player_path in config.json")

// ErrNoLogin means librespot hasn't been signed in yet.
var ErrNoLogin = errors.New("the player isn't signed in: run 'waxon player login'")

// Dir is where librespot keeps its credentials and volume, beside waxon's
// own token.
func Dir() string {
	return filepath.Join(config.Dir(), "librespot")
}

func credentialsPath(dir string) string {
	return filepath.Join(dir, "credentials.json")
}

// LoggedIn reports whether librespot has cached credentials in dir.
func LoggedIn(dir string) bool {
	_, err := os.Stat(credentialsPath(dir))
	return err == nil
}

// Logout removes librespot's cached login. A missing one is not an error.
func Logout(dir string) error {
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Find returns the librespot binary: path if set, else librespot on PATH.
func Find(path string) (string, error) {
	if path == "" {
		path = Backend
	}
	bin, err := exec.LookPath(path)
	if err != nil {
		return "", ErrNotInstalled
	}
	return bin, nil
}

// DeviceName is the Connect device name waxon's player uses on this
// machine, so players on different machines can be told apart.
func DeviceName() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		return "waxon"
	}
	host, _, _ = strings.Cut(host, ".")
	return "waxon (" + host + ")"
}

// baseArgs are the flags every run shares: a fixed name, no zeroconf
// (nobody else should be able to take the device over), and the login kept
// in dir. No --cache, so no audio is cached.
func baseArgs(dir, name string) []string {
	return []string{"--name", name, "--disable-discovery", "--system-cache", dir}
}

// complete reports whether a whole credentials file is in dir (librespot
// may still be writing it).
func complete(dir string) bool {
	data, err := os.ReadFile(credentialsPath(dir))
	return err == nil && len(data) > 0 && json.Valid(data)
}

// Login runs librespot's browser sign-in and returns once its credentials
// are cached in dir, replacing any login there only when it succeeds. With
// paste set (SSH sessions), librespot asks for the address the browser lands
// on instead of listening for it. librespot's prompts go to out; in is passed
// through for the paste.
func Login(ctx context.Context, bin, dir string, paste bool, in io.Reader, out io.Writer) error {
	// librespot skips the sign-in when it finds cached credentials, so sign
	// in from an empty directory and move the result into place.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dir, "login-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := signIn(ctx, bin, tmp, paste, in, out); err != nil {
		return err
	}
	return os.Rename(credentialsPath(tmp), credentialsPath(dir))
}

func signIn(ctx context.Context, bin, dir string, paste bool, in io.Reader, out io.Writer) error {
	args := append(baseArgs(dir, DeviceName()), "--enable-oauth")
	if paste {
		args = append(args, "--oauth-port", "0")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // the user's own librespot
	cmd.Stdin, cmd.Stdout, cmd.Stderr = in, out, out
	if err := cmd.Start(); err != nil {
		return err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	// librespot carries on as a player after signing in; stop it once the
	// credentials are on disk.
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			if complete(dir) {
				return nil
			}
			if err == nil {
				err = errors.New("librespot exited")
			}
			return fmt.Errorf("sign-in didn't finish: %w", err)
		case <-tick.C:
			if complete(dir) {
				cancel()
				<-exited
				return nil
			}
		case <-ctx.Done():
			<-exited
			return ctx.Err()
		}
	}
}

// Sidecar is a running librespot device.
type Sidecar struct {
	cmd  *exec.Cmd
	done chan struct{}

	mu       sync.Mutex
	err      error  // why it exited, once it has
	lastLine string // last thing librespot logged, for error messages
	stopping bool
}

// Start launches librespot from the cached login in dir as device name.
// Its log goes to logw (and the last line is kept for Err). onExit, if set,
// is called when it exits without Stop having been called.
func Start(bin, dir, name string, logw io.Writer, onExit func(error)) (*Sidecar, error) {
	if !LoggedIn(dir) {
		return nil, ErrNoLogin
	}
	s := &Sidecar{done: make(chan struct{})}
	s.cmd = exec.Command(bin, baseArgs(dir, name)...) //nolint:gosec // the user's own librespot
	s.cmd.Stdout = logw
	s.cmd.Stderr = &lastLineWriter{w: logw, s: s}
	if err := s.cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := s.cmd.Wait()
		s.mu.Lock()
		stopping := s.stopping
		if !stopping {
			s.err = exitError(err, s.lastLine)
		}
		exitErr := s.err
		s.mu.Unlock()
		close(s.done)
		if !stopping && onExit != nil {
			onExit(exitErr)
		}
	}()
	return s, nil
}

// exitError describes an unexpected exit, preferring librespot's own last
// words (e.g. "Login failed with reason: Bad credentials").
func exitError(err error, last string) error {
	if last != "" {
		if _, msg, ok := strings.Cut(last, "] "); ok {
			last = msg
		}
		return fmt.Errorf("librespot stopped: %s", last)
	}
	if err == nil {
		return errors.New("librespot stopped")
	}
	return fmt.Errorf("librespot stopped: %w", err)
}

// Stop ends librespot, waiting briefly for it to leave Spotify cleanly
// before killing it.
func (s *Sidecar) Stop() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	select {
	case <-s.done:
		return
	default:
	}
	if err := interrupt(s.cmd.Process); err != nil {
		_ = s.cmd.Process.Kill()
	}
	select {
	case <-s.done:
	case <-time.After(2 * time.Second):
		_ = s.cmd.Process.Kill()
		<-s.done
	}
}

// lastLineWriter forwards librespot's log and remembers its last error
// line.
type lastLineWriter struct {
	w io.Writer
	s *Sidecar
}

func (l *lastLineWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimSpace(string(p)), "\n") {
		if strings.Contains(line, "ERROR") {
			l.s.mu.Lock()
			l.s.lastLine = strings.TrimSpace(line)
			l.s.mu.Unlock()
		}
	}
	return l.w.Write(p)
}
