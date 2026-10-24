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

// ErrNotInstalled means the librespot binary couldn't be found. Find wraps
// it with how to install librespot here.
var ErrNotInstalled = errors.New("librespot isn't installed")

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
		return "", fmt.Errorf("%w: install it with '%s' (or set player_path in config.json)", ErrNotInstalled, InstallCommand())
	}
	return bin, nil
}

// Installer is how to install librespot with a package manager on this
// machine. Args is nil when none was found; Command is then a manual hint.
type Installer struct {
	Manager string   // "Homebrew", "pacman", …
	Args    []string // the command to run, e.g. ["brew", "install", "librespot"]
	Command string   // Args as typed, or what to do by hand
}

// FindInstaller picks the first package manager found that ships librespot,
// falling back to cargo (librespot is a Rust program).
func FindInstaller() Installer {
	for _, pm := range []Installer{
		{Manager: "Homebrew", Args: []string{"brew", "install", "librespot"}},
		{Manager: "pacman", Args: []string{"sudo", "pacman", "-S", "--needed", "librespot"}},
		{Manager: "Nix", Args: []string{"nix", "profile", "install", "nixpkgs#librespot"}},
		{Manager: "cargo", Args: []string{"cargo", "install", "librespot"}},
	} {
		bin := pm.Args[0]
		if bin == "sudo" {
			bin = pm.Args[1]
		}
		if _, err := exec.LookPath(bin); err == nil {
			pm.Command = strings.Join(pm.Args, " ")
			return pm
		}
	}
	return Installer{Command: "install Rust (https://rustup.rs), then: cargo install librespot"}
}

// InstallCommand is FindInstaller's command, for messages.
func InstallCommand() string { return FindInstaller().Command }

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

// playArgs are the flags for running the device: baseArgs, at the highest
// quality Spotify streams to Premium accounts (librespot defaults to 160).
func playArgs(dir, name string) []string {
	return append(baseArgs(dir, name), "--bitrate", "320")
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
	// librespot writes it world-readable; it's a reusable login, so keep it
	// to the user like waxon's own token.
	if err := os.Chmod(credentialsPath(tmp), 0o600); err != nil {
		return err
	}
	return os.Rename(credentialsPath(tmp), credentialsPath(dir))
}

// Secure tightens a login cached before Login set its permissions.
func Secure(dir string) {
	_ = os.Chmod(credentialsPath(dir), 0o600)
}

func signIn(ctx context.Context, bin, dir string, paste bool, in io.Reader, out io.Writer) error {
	args := append(baseArgs(dir, DeviceName()), "--enable-oauth")
	if paste {
		args = append(args, "--oauth-port", "0")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // the user's own librespot
	cmd.Stdout, cmd.Stderr = out, out
	// Give librespot a real pipe and copy in into it ourselves: os/exec's
	// own copy of a non-file Stdin is waited on by Wait, and in may be a
	// pipe nothing is ever written to (the setup's paste field).
	switch f, ok := in.(*os.File); {
	case in == nil:
	case ok:
		cmd.Stdin = f
	default:
		r, w, err := os.Pipe()
		if err != nil {
			return err
		}
		cmd.Stdin = r
		defer func() { _ = r.Close(); _ = w.Close() }()
		go func() { _, _ = io.Copy(w, in) }()
	}
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
	s.cmd = exec.Command(bin, playArgs(dir, name)...) //nolint:gosec // the user's own librespot
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
