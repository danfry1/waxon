package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"

	"github.com/danfry1/waxon/app"
	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/config"
	"github.com/danfry1/waxon/player"
	"github.com/danfry1/waxon/source"
	myspotify "github.com/danfry1/waxon/spotify"
)

// waxon doctor: one line per thing waxon depends on, each with the fix when
// it's wrong. Everything it touches comes in through doctorEnv, for tests.

type checkStatus int

const (
	checkOK checkStatus = iota
	checkInfo
	checkWarn
	checkFail
)

type check struct {
	name   string
	status checkStatus
	detail string
	fix    string
}

type doctorEnv struct {
	configPath string
	config     func() (config.Config, error)
	envID      string // $SPOTIFY_CLIENT_ID
	tokenPath  string
	// profile and devices talk to Spotify with the saved login.
	profile    func(ctx context.Context) (name string, err error)
	devices    func(ctx context.Context) ([]source.Device, error)
	lyrics     func(ctx context.Context) error
	player     func(path string) (bin, version string, err error)
	playerDir  string
	playerName string
	colour     string // "true colour", "256 colours", "no colour"; "" when not a terminal
	cacheDir   string
	now        func() time.Time
	exists     func(path string) bool
}

func runDoctor(ctx context.Context, env doctorEnv) []check {
	var checks []check
	add := func(c check) { checks = append(checks, c) }

	// Config and which Spotify app.
	cfg, err := env.config()
	if err != nil {
		add(check{"Config", checkFail, err.Error(), "fix or delete " + env.configPath})
	} else {
		add(check{"Config", checkOK, env.configPath, ""})
	}
	// The logins live beside config.json: warn when that folder would
	// leave this machine with a dotfiles repo or a sync service.
	if where, fix := exposure(filepath.Dir(env.configPath), env.exists); where != "" {
		add(check{"Logins", checkWarn, "waxon's logins are in " + where + ", so they could leave this machine", fix})
	}
	clientID := cfg.ClientID
	switch {
	case env.envID != "":
		add(check{"Spotify app", checkOK, "from $SPOTIFY_CLIENT_ID (" + short(env.envID) + ")", ""})
	case clientID == "" || clientID == myauth.DefaultClientID:
		add(check{
			"Spotify app", checkWarn, "the shared app, which Spotify throttles for every waxon user at once",
			"waxon auth --own sets up your own (about 2 minutes, no rate limits)",
		})
	default:
		add(check{"Spotify app", checkOK, "your own (" + short(clientID) + ")", ""})
	}

	// The saved login, and whether Spotify accepts it.
	signedIn := false
	info, err := os.Stat(env.tokenPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		add(check{"Signed in", checkFail, "no saved Spotify login", "run waxon (or waxon auth) to sign in"})
	case err != nil:
		add(check{"Signed in", checkFail, err.Error(), "waxon auth"})
	default:
		start := env.now()
		lookup, cancel := context.WithTimeout(ctx, 10*time.Second)
		name, perr := env.profile(lookup)
		cancel()
		took := env.now().Sub(start).Round(time.Millisecond)
		switch {
		case perr == nil:
			signedIn = true
			who := "signed in"
			if name != "" {
				who = "signed in as " + name
			}
			add(check{"Signed in", checkOK, fmt.Sprintf("%s (Spotify answered in %s)", who, took), ""})
		case errors.Is(perr, errBadLogin):
			add(check{"Signed in", checkFail, "the saved login can't be read: " + perr.Error(), "waxon auth signs in again"})
		case httpStatus(perr) == http.StatusUnauthorized || httpStatus(perr) == http.StatusBadRequest:
			add(check{"Signed in", checkFail, "Spotify no longer accepts the saved login", "waxon auth signs in again"})
		case httpStatus(perr) == http.StatusTooManyRequests:
			add(check{"Signed in", checkWarn, "Spotify is rate-limiting this app right now", "wait a little; waxon auth --own avoids it for good"})
		default:
			add(check{"Signed in", checkFail, "couldn't reach Spotify: " + perr.Error(), "check your internet connection"})
		}
		if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
			add(check{
				"Login file", checkWarn, fmt.Sprintf("%s is readable by other users (%v)", env.tokenPath, info.Mode().Perm()),
				"chmod 600 " + env.tokenPath,
			})
		}
	}

	// Devices to play on.
	playerOn := cfg.Player == player.Backend
	if signedIn {
		lookup, cancel := context.WithTimeout(ctx, 10*time.Second)
		devices, derr := env.devices(lookup)
		cancel()
		switch {
		case derr != nil:
			add(check{"Devices", checkWarn, "couldn't list them: " + derr.Error(), ""})
		case len(devices) == 0 && playerOn:
			add(check{"Devices", checkInfo, "none open; waxon's player appears while waxon runs", ""})
		case len(devices) == 0:
			add(check{
				"Devices", checkWarn, "no Spotify app is open on your account",
				"open Spotify on any device, or play right here: waxon player login",
			})
		default:
			names := make([]string, len(devices))
			for i, d := range devices {
				names[i] = d.Name
			}
			add(check{"Devices", checkOK, strings.Join(names, " · "), ""})
		}
	}

	// waxon's own player (optional).
	bin, version, perr := env.player(cfg.PlayerPath)
	loggedIn := player.LoggedIn(env.playerDir)
	switch {
	case !playerOn && !loggedIn:
		add(check{"Player", checkInfo, "off (optional: plays with no Spotify app open)", "waxon player login sets it up"})
	case !playerOn:
		add(check{"Player", checkInfo, "signed in but off", "waxon player login turns it on"})
	case errors.Is(perr, player.ErrNotInstalled):
		add(check{"Player", checkFail, "on, but librespot isn't installed", "install it with: " + player.InstallCommand()})
	case perr != nil:
		add(check{"Player", checkFail, "on, but " + perr.Error(), "reinstall it with: " + player.InstallCommand()})
	case !loggedIn:
		add(check{"Player", checkFail, "on, but not signed in", "waxon player login"})
	default:
		add(check{"Player", checkOK, fmt.Sprintf("%s (%s), plays as %q", version, bin, env.playerName), ""})
	}
	if creds := filepath.Join(env.playerDir, "credentials.json"); runtime.GOOS != "windows" {
		if info, err := os.Stat(creds); err == nil && info.Mode().Perm()&0o077 != 0 {
			add(check{"Player login", checkWarn, fmt.Sprintf("%s is readable by other users (%v)", creds, info.Mode().Perm()), "chmod 600 " + creds})
		}
	}

	// Lyrics come from lrclib.net.
	lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
	if err := env.lyrics(lookup); err != nil {
		add(check{"Lyrics", checkWarn, "lrclib.net isn't reachable, so lyrics won't load: " + err.Error(), ""})
	} else {
		add(check{"Lyrics", checkOK, "lrclib.net reachable", ""})
	}
	cancel()

	// Terminal (skipped when the output isn't one).
	switch env.colour {
	case "":
	case "true colour":
		add(check{"Terminal", checkOK, env.colour, ""})
	default:
		add(check{
			"Terminal", checkInfo, env.colour + "; album art and themes look best with true colour",
			"set COLORTERM=truecolor if your terminal supports it",
		})
	}

	// Cache.
	add(check{"Cache", checkInfo, fmt.Sprintf("%s (%s)", env.cacheDir, humanSize(dirSize(env.cacheDir))), ""})
	return checks
}

func printDoctor(w io.Writer, checks []check) (failed int) {
	mark := map[checkStatus]string{
		checkOK:   lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓"),
		checkInfo: lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("·"),
		checkWarn: lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render("!"),
		checkFail: lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("✗"),
	}
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	warned := 0
	fmt.Fprintln(w, "waxon doctor")
	fmt.Fprintln(w)
	for _, c := range checks {
		fmt.Fprintf(w, "  %s %-12s %s\n", mark[c.status], c.name, c.detail)
		if c.fix != "" && c.status != checkOK {
			fmt.Fprintf(w, "    %-12s %s\n", "", dim.Render("→ "+c.fix))
		}
		switch c.status {
		case checkFail:
			failed++
		case checkWarn:
			warned++
		}
	}
	fmt.Fprintln(w)
	switch {
	case failed > 0:
		fmt.Fprintf(w, "%d %s to fix (✗).\n", failed, plural(failed, "thing", "things"))
	case warned > 0:
		fmt.Fprintf(w, "Working, with %d %s worth a look (!).\n", warned, plural(warned, "thing", "things"))
	default:
		fmt.Fprintln(w, "All good.")
	}
	return failed
}

// syncFolders are path parts of folders that sync services upload.
var syncFolders = []struct{ part, name string }{
	{"Dropbox", "Dropbox"},
	{"OneDrive", "OneDrive"},
	{"Google Drive", "Google Drive"},
	{filepath.Join("Library", "Mobile Documents"), "iCloud Drive"},
	{filepath.Join("Library", "CloudStorage"), "a cloud-storage folder"},
}

// exposure reports where dir would be copied off the machine: a git
// repository (a dotfiles repo, say) or a synced folder, with the fix.
func exposure(dir string, exists func(string) bool) (where, fix string) {
	for d := dir; ; d = filepath.Dir(d) {
		if exists(filepath.Join(d, ".git")) {
			return "the git repository at " + d, "add " + relOr(d, dir) + "/ to its .gitignore (or move the repository)"
		}
		if parent := filepath.Dir(d); parent == d {
			break
		}
	}
	for _, f := range syncFolders {
		if strings.Contains(dir, string(filepath.Separator)+f.part+string(filepath.Separator)) ||
			strings.HasSuffix(dir, string(filepath.Separator)+f.part) {
			return f.name, "set XDG_CONFIG_HOME to a folder that isn't synced, then run waxon auth"
		}
	}
	return "", ""
}

// relOr is path relative to base, or path itself.
func relOr(base, path string) string {
	if rel, err := filepath.Rel(base, path); err == nil {
		return rel
	}
	return path
}

// errBadLogin marks a saved login that couldn't be read at all.
var errBadLogin = errors.New("unreadable login")

// runDoctorCommand is `waxon doctor`.
func runDoctorCommand() int {
	cacheDir := filepath.Dir(app.DefaultSnapshotPath())
	env := doctorEnv{
		configPath: config.DefaultPath(),
		config:     config.Load,
		envID:      os.Getenv("SPOTIFY_CLIENT_ID"),
		tokenPath:  myauth.DefaultTokenPath(),
		profile: func(ctx context.Context) (string, error) {
			src, err := savedSource()
			if err != nil {
				return "", err
			}
			_, name, err := src.Profile(ctx)
			return name, err
		},
		devices: func(ctx context.Context) ([]source.Device, error) {
			src, err := savedSource()
			if err != nil {
				return nil, err
			}
			return src.Devices(ctx)
		},
		lyrics:     pingLyrics,
		player:     librespotVersion,
		playerDir:  player.Dir(),
		playerName: player.DeviceName(),
		colour:     terminalColour(),
		cacheDir:   cacheDir,
		now:        time.Now,
		exists: func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		},
	}
	if printDoctor(os.Stdout, runDoctor(context.Background(), env)) > 0 {
		return 1
	}
	return 0
}

// savedSource is a Spotify client for the saved login.
func savedSource() (*myspotify.PlayerSource, error) {
	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errBadLogin, err)
	}
	return myspotify.NewPlayerSource(myspotify.NewClient(effectiveClientID(), token, tokenPath)), nil
}

func pingLyrics(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, "https://lrclib.net/", nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("it answered %s", resp.Status)
	}
	return nil
}

func librespotVersion(path string) (bin, version string, err error) {
	bin, err = player.Find(path)
	if err != nil {
		return "", "", err
	}
	out, err := exec.Command(bin, "--version").Output() //nolint:gosec // the user's own librespot
	if err != nil {
		return bin, "", fmt.Errorf("%s doesn't run (%w)", bin, err)
	}
	version = "librespot"
	if f := strings.Fields(string(out)); len(f) >= 2 {
		version = f[0] + " " + f[1]
	}
	return bin, version, nil
}

// terminalColour describes stdout's colour support ("" when it isn't a
// terminal).
func terminalColour() string {
	fd := os.Stdout.Fd()
	if !isatty.IsTerminal(fd) && !isatty.IsCygwinTerminal(fd) {
		return ""
	}
	return colourName(app.DetectArtMode())
}

func colourName(m app.ArtMode) string {
	switch m {
	case app.ArtTrueColor:
		return "true colour"
	case app.ArtANSI256:
		return "256 colours"
	}
	return "no colour"
}

// httpStatus digs an HTTP status out of a Spotify API error (0 if none).
func httpStatus(err error) int {
	var s interface{ HTTPStatus() int }
	if errors.As(err, &s) {
		return s.HTTPStatus()
	}
	return 0
}

func short(id string) string {
	if len(id) > 8 {
		return id[:8] + "…"
	}
	return id
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			if info, err := d.Info(); err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
