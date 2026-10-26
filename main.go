package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/danfry1/waxon/app"
	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/cli"
	"github.com/danfry1/waxon/config"
	"github.com/danfry1/waxon/media"
	"github.com/danfry1/waxon/player"
	myspotify "github.com/danfry1/waxon/spotify"
	"github.com/danfry1/waxon/update"
)

// version is set at build time via ldflags:
//
//	go build -ldflags "-X main.version=v1.2.3"
var version = "dev"

func main() {
	cleanup := initLogging()
	defer cleanup()
	useSystemProxy() // before anything goes on the network: Go reads it once
	if err := config.MigrateDir(); err != nil {
		slog.Warn("config stays in its old place", "error", err)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "auth":
			opts, err := parseAuthFlags(os.Args[2:])
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				cleanup()
				os.Exit(2) //nolint:gocritic // cleanup called above
			}
			if interactiveTerminal() {
				if code := runAuthSetup(opts); code != 0 {
					cleanup()
					os.Exit(code)
				}
				return
			}
			runAuthWith(newOnboarding(), opts)
			return
		case "logout":
			if err := app.RemoveSnapshot(app.DefaultSnapshotPath()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not remove the library cache: %v\n", err)
			}
			if err := player.Logout(player.Dir()); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not remove the player's login: %v\n", err)
			} else if cfg, _ := config.Load(); cfg.Player == player.Backend {
				_ = config.Save(config.Config{Player: "off"}) // 'waxon player login' turns it back on
			}
			if err := myauth.DeleteToken(myauth.DefaultTokenPath()); err != nil {
				fmt.Fprintln(os.Stderr, err)
				cleanup()
				os.Exit(1) //nolint:gocritic // cleanup called above
			}
			fmt.Println("Signed out of Spotify. Quit any running waxon too, then run 'waxon auth' to connect again.")
			return
		case "doctor":
			code := runDoctorCommand()
			cleanup()
			os.Exit(code) //nolint:gocritic // cleanup called above
		case "player":
			code := runPlayer(os.Args[2:])
			cleanup()
			os.Exit(code) //nolint:gocritic // cleanup called above
		case "debug":
			runDebug()
			return
		case "version", "--version", "-v":
			fmt.Println("waxon " + version)
			// What the last launch found, on stderr: no network, and the
			// output scripts read stays one line.
			if cfg, _ := config.Load(); updateCheckOn(cfg) {
				if v := update.Known(version, updateStatePath()); v != "" {
					fmt.Fprintln(os.Stderr, newVersionLine(v))
				}
			}
			return
		case "demo":
			runDemo()
			return
		case "help", "--help", "-h":
			printUsage()
			return
		case "themes":
			for _, name := range app.ThemeNames() {
				fmt.Println(name)
			}
			return
		case "config":
			fmt.Println(config.DefaultPath())
			return
		}
		if cli.IsCommand(os.Args[1]) {
			src, ok := newSource()
			if !ok {
				cleanup()
				os.Exit(1) //nolint:gocritic // cleanup called above
			}
			r := &cli.Runner{Src: src, Out: os.Stdout, Err: os.Stderr}
			code := r.Run(context.Background(), os.Args[1:])
			cleanup()
			os.Exit(code) //nolint:gocritic // cleanup called above
		}
	}

	src, ok := newSource()
	if !ok {
		cleanup()
		os.Exit(1) //nolint:gocritic // cleanup called above
	}

	opts := applyUserConfig()
	opts.SharedClientID = effectiveClientID() == myauth.DefaultClientID
	cfg, _ := config.Load()
	runner, problem := newPlayerRunner(cfg)
	if runner != nil {
		opts.PlayerName = runner.name
	}
	opts.PlayerProblem = problem
	opts.OpenSpotifyApp = spotifyAppLauncher()
	opts.LyricsOff = cfg.Lyrics != nil && !*cfg.Lyrics
	opts.ThisComputer = thisComputerNames()
	var p *tea.Program
	deps := setupDeps("", "")
	opts.PlayerSetup = &deps
	// Set up from the device picker: start the player now, as a launch with
	// it turned on would have.
	opts.StartPlayer = func() string {
		latest, _ := config.Load() // the player was turned on since launch
		r, _ := newPlayerRunner(latest)
		if r == nil {
			return ""
		}
		runner.stop() // a previous one that died
		runner = r
		r.launch(context.Background(), src, p, false)
		return r.name
	}
	mediaKeys := (cfg.MediaKeys == nil || *cfg.MediaKeys) && !overSSH()
	run := func() int {
		// Media keys and the system's Now Playing (media package); presses
		// reach the program like key presses.
		var prog atomic.Pointer[tea.Program]
		if mediaKeys {
			ctl, err := media.Start(func(e media.Event) {
				if running := prog.Load(); running != nil {
					running.Send(app.MediaMsg(e))
				}
			})
			switch {
			case err == nil:
				opts.Media = ctl
				defer ctl.Close()
			case !errors.Is(err, media.ErrUnsupported):
				slog.Warn("media keys unavailable", "error", err)
			}
		}
		m := app.NewModel(src).WithOptions(opts)
		p = tea.NewProgram(m, programOptions()...)
		prog.Store(p)
		runner.start(context.Background(), src, p)
		final, err := p.Run()
		runner.stop()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			return 1
		}
		if fm, ok := final.(app.Model); ok && fm.NewVersion() != "" {
			fmt.Println(newVersionLine(fm.NewVersion()))
		}
		return 0
	}
	if !mediaKeys {
		os.Exit(run())
	}
	// On macOS the media controls need the main thread's event loop, so the
	// TUI runs beside it.
	os.Exit(media.RunMain(run))
}

// graphicsOut is the Program's output when kitty art is on (see
// configureArt): images are written through it between frames.
var graphicsOut *app.TTYOutput

// programOptions are the Bubbletea options for the TUI.
func programOptions() []tea.ProgramOption {
	opts := []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion(), tea.WithReportFocus()}
	if graphicsOut != nil {
		opts = append(opts, tea.WithOutput(graphicsOut))
	}
	return opts
}

// configureArt applies the "art" setting (from origin, for messages) on top
// of the detected colour support. Kitty images need true colour: the image
// ID travels as a 24-bit colour.
func configureArt(setting, origin string) {
	mode := app.DetectArtMode()
	app.SetArtMode(mode)
	switch setting {
	case "", "blocks":
		return
	case "auto":
		if mode != app.ArtTrueColor || !app.KittyTerminal() {
			return
		}
	case "kitty":
		if mode != app.ArtTrueColor {
			fmt.Fprintf(os.Stderr, "Warning: %s art \"kitty\" needs a true-colour terminal (set COLORTERM=truecolor if yours is); using block art\n", origin)
			return
		}
	default:
		fmt.Fprintf(os.Stderr, "Warning: %s art %q not recognised (use blocks, kitty or auto)\n", origin, setting)
		return
	}
	graphicsOut = app.NewTTYOutput(os.Stdout)
	app.SetGraphicsOutput(graphicsOut)
	app.SetArtMode(app.ArtKitty)
}

// applyUserConfig reads theme/colour/key settings from the config file,
// applies the theme and art mode globally, and returns the Model options.
// Problems are reported on stderr and fall back to defaults rather than
// refusing to start — a typo in config.json shouldn't lock anyone out.
func applyUserConfig() app.Options {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v (using defaults)\n", err)
	}
	var opts app.Options

	overrides, err := app.PaletteFromMap(cfg.Colors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: config colors: %v (ignored)\n", err)
		overrides = app.Palette{}
	}
	opts.ColorOverrides = overrides
	palette, err := app.ResolveTheme(cfg.Theme, overrides)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v (using %s)\n", err, app.DefaultThemeName)
		palette, _ = app.ResolveTheme(app.DefaultThemeName, overrides)
	}
	app.ApplyTheme(palette)
	art, origin := cfg.Art, "config"
	if env := os.Getenv("WAXON_ART"); env != "" {
		art, origin = env, "WAXON_ART"
	}
	configureArt(art, origin)
	opts.SnapshotPath = app.DefaultSnapshotPath()
	if dir := app.DefaultImageCacheDir(); dir != "" {
		app.SetImageCacheDir(dir)
		go app.PruneImageCache()
	}

	if len(cfg.Keys) > 0 {
		km, err := app.KeyMapFromOverrides(cfg.Keys)
		opts.Keys = &km
		if err != nil {
			// Shown in the app: stderr is hidden behind the full-screen UI.
			opts.KeysProblem = err.Error()
		}
	}
	opts.SaveTheme = func(name string) error {
		return config.Save(config.Config{Theme: name})
	}
	opts.Settings = app.Settings{
		Theme:        cmp.Or(cfg.Theme, app.DefaultThemeName),
		Art:          cmp.Or(cfg.Art, "blocks"),
		MediaKeysOff: cfg.MediaKeys != nil && !*cfg.MediaKeys,
		CustomKeys:   len(cfg.Keys),
		Path:         config.DefaultPath(),
	}
	if env := os.Getenv("WAXON_ART"); env != "" {
		opts.Settings.ArtNote = "WAXON_ART=" + env + " overrides it"
	}
	if overSSH() {
		opts.Settings.MediaKeysNote = "off over SSH"
	}
	opts.Settings.UpdateCheckOff = !updateCheckOn(cfg)
	opts.Version = version
	opts.UpgradeCommand = upgradeCommand()
	if updateCheckOn(cfg) {
		opts.CheckUpdate = func() string {
			return update.Available(context.Background(), version, updateStatePath())
		}
	}
	opts.SaveSetting = saveSetting
	return opts
}

// updateCheckOn: the config hasn't turned update checks off.
func updateCheckOn(cfg config.Config) bool {
	return cfg.UpdateCheck == nil || *cfg.UpdateCheck
}

// updateStatePath is where the last update check is kept, beside the
// library cache.
func updateStatePath() string {
	dir := app.DefaultImageCacheDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(dir), "update.json")
}

// upgradeCommand is how this copy of waxon was installed, as the command
// that upgrades it ("" when that can't be told).
func upgradeCommand() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return update.Command(exe)
}

// newVersionLine tells the terminal a newer release is out and how to get it.
func newVersionLine(v string) string {
	how := upgradeCommand()
	if how == "" {
		how = update.ReleasesURL
	}
	return fmt.Sprintf("waxon %s is out (this is %s). Upgrade: %s", v, strings.TrimPrefix(version, "v"), how)
}

// saveSetting saves one setting from the settings overlay by its
// config.json name; config.Save merges it into the file.
func saveSetting(name, value string) error {
	on := value == "on"
	switch name {
	case "theme":
		return config.Save(config.Config{Theme: value})
	case "art":
		return config.Save(config.Config{Art: value})
	case "media_keys":
		return config.Save(config.Config{MediaKeys: &on})
	case "lyrics":
		return config.Save(config.Config{Lyrics: &on})
	case "update_check":
		return config.Save(config.Config{UpdateCheck: &on})
	}
	return fmt.Errorf("unknown setting %q", name)
}

// newSource builds the Spotify-backed source from the saved token. On failure
// it prints guidance and returns ok=false (the caller exits).
func newSource() (*myspotify.PlayerSource, bool) {
	clientID := resolveClientID()
	if clientID == "" {
		clientID = myauth.DefaultClientID
	}

	// Warn if the token was issued for a different client ID.
	if saved, err := config.Load(); err == nil && saved.ClientID != "" && saved.ClientID != clientID {
		fmt.Fprintf(os.Stderr, "Warning: current client ID differs from the one used during auth.\n")
		fmt.Fprintf(os.Stderr, "Run 'waxon auth' to re-authenticate, or set SPOTIFY_CLIENT_ID=%s\n", saved.ClientID)
	}

	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// First run: go straight into setup instead of bouncing the user
			// to a command they haven't heard of yet.
			if interactiveTerminal() {
				s := runSetup(setupDeps(os.Getenv("SPOTIFY_CLIENT_ID"), ""), app.SetupFull)
				switch {
				case !s.Connected():
					fmt.Println("Setup stopped. Run 'waxon' again whenever you're ready.")
					return nil, false
				case !s.Finished():
					// Signed in, then left before the end: don't open the
					// app they just quit out of.
					fmt.Println("Connected to Spotify. Run 'waxon' to start.")
					os.Exit(0)
				}
				clientID = effectiveClientID()
				token, err = myauth.LoadToken(tokenPath)
			} else if o := newOnboarding(); o.interactive {
				runAuthFlow(o, authOptions{}, true)
				token, err = myauth.LoadToken(tokenPath)
			}
		}
	}
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintln(os.Stderr, "No Spotify token found. Run 'waxon auth' to connect your account.")
		} else {
			fmt.Fprintf(os.Stderr, "Failed to read token: %v\n", err)
			fmt.Fprintln(os.Stderr, "Your token file may be corrupted. Run 'waxon auth' to re-authenticate.")
		}
		return nil, false
	}

	cp := myspotify.NewClient(clientID, token, tokenPath)
	return myspotify.NewPlayerSource(cp), true
}

// effectiveClientID is the client ID waxon will use: the configured one, or
// the bundled shared ID when none is set.
func effectiveClientID() string {
	if id := resolveClientID(); id != "" {
		return id
	}
	return myauth.DefaultClientID
}

// resolveClientID returns the Client ID from the env var (takes priority)
// or from the saved config file. Returns "" if neither is set.
func resolveClientID() string {
	if id := os.Getenv("SPOTIFY_CLIENT_ID"); id != "" {
		return id
	}
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	return cfg.ClientID
}

// authenticateWith runs the OAuth flow for clientID and saves the token and
// client ID.
func authenticateWith(clientID string) {
	token, err := myauth.Authenticate(clientID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Authentication failed: %v\n", err)
		os.Exit(1)
	}

	if err := saveLogin(clientID, token); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to save the login: %v\n", err)
		os.Exit(1)
	}
}

func runDebug() {
	clientID := resolveClientID()
	if clientID == "" {
		clientID = myauth.DefaultClientID
		fmt.Println("Client ID: (default)")
	} else {
		fmt.Printf("Client ID: %s\n", clientID)
	}
	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		fmt.Println("No token:", err)
		return
	}
	cp := myspotify.NewClient(clientID, token, tokenPath)
	src := myspotify.NewPlayerSource(cp)

	ctx := context.Background()
	ps, err := src.CurrentPlayback(ctx)
	if err != nil {
		fmt.Println("CurrentPlayback error:", err)
		return
	}
	if ps == nil || ps.Track == nil {
		fmt.Println("No track playing")
		return
	}
	track := ps.Track
	fmt.Printf("Track: %s - %s (ArtistID=%s, AlbumID=%s)\n", track.Name, track.Artist, track.ArtistID, track.AlbumID)

	if track.ArtistID != "" {
		fmt.Printf("\nGetArtist(%s)...\n", track.ArtistID)
		page, err := src.GetArtist(ctx, track.ArtistID)
		if err != nil {
			fmt.Println("ERROR:", err)
		} else {
			fmt.Printf("Artist: %s, Genres: %v, Tracks: %d\n", page.Name, page.Genres, len(page.Tracks))
		}
	}

	if track.AlbumID != "" {
		fmt.Printf("\nGetAlbum(%s)...\n", track.AlbumID)
		page, err := src.GetAlbum(ctx, track.AlbumID)
		if err != nil {
			fmt.Println("ERROR:", err)
		} else {
			fmt.Printf("Album: %s — %s, Tracks: %d\n", page.Name, page.Artist, len(page.Tracks))
		}
	}
}

// initLogging configures the global slog logger and returns a cleanup function
// that closes the log file. The caller must defer the cleanup.
// If WAXON_LOG is set to a file path, debug-level logs are written there;
// otherwise logging is disabled.
func initLogging() func() {
	logPath := os.Getenv("WAXON_LOG")
	if logPath == "" {
		slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
		return func() {}
	}
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: cannot open log file: %v\n", err)
		return func() {}
	}
	handler := slog.NewTextHandler(f, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
	slog.Info("waxon starting", "version", version)
	return func() { _ = f.Close() }
}

func printUsage() {
	fmt.Println(`waxon — vim-modal Spotify terminal client

Usage:
  waxon          Launch the TUI
  waxon auth     Connect your Spotify account (guided; --own | --shared | --client-id ID)
                 Over SSH, paste the address the browser ends up on when asked
  waxon logout   Forget the saved Spotify and player logins (keeps the rest of your config)
  waxon player   waxon's own player via librespot: status, or login | off
  waxon doctor   Check the login, Spotify app, devices, player and more
  waxon version  Print version
  waxon themes   List colour themes (set with "theme" in config.json or :theme)
  waxon config   Print the config file path
  waxon help     Show this help
` + demoUsage + `
` + cli.Usage() + `

Environment:
  SPOTIFY_CLIENT_ID  Override the saved Client ID
  WAXON_LOG          Path to debug log file (e.g. /tmp/waxon.log)
  WAXON_ART          Album art: blocks (default), kitty, or auto`)
}

// overSSH reports whether waxon is running in an SSH session, where the
// machine's media keys and Now Playing belong to whoever is at it.
func overSSH() bool {
	return os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != ""
}
