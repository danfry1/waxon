package main

import (
	"context"
	"fmt"
	"io"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-isatty"
	"golang.org/x/oauth2"

	"github.com/danfry1/waxon/app"
	myauth "github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/config"
	"github.com/danfry1/waxon/player"
	"github.com/danfry1/waxon/source"
	myspotify "github.com/danfry1/waxon/spotify"
)

// setupDeps wires the guided setup (app.Setup) to auth, config, the Spotify
// API and the player. clientID, when set, skips the choice of Spotify app.
func setupDeps(clientID, note string) app.SetupDeps {
	return app.SetupDeps{
		ClientID:     clientID,
		ClientIDNote: note,
		SharedID:     myauth.DefaultClientID,
		Remote:       myauth.IsRemoteSession(),
		RedirectURI:  myauth.RedirectURI,
		DashboardURL: DashboardURL,
		BeginLogin: func(id string) (app.SetupLogin, error) {
			l, err := myauth.BeginLogin(id)
			if err != nil {
				return nil, err
			}
			return setupLogin{l: l, clientID: id}, nil
		},
		Devices:     savedAccountDevices,
		Player:      setupPlayer{},
		OpenBrowser: myauth.OpenBrowser,
	}
}

// setupLogin is an auth.Login that saves the account when it completes.
type setupLogin struct {
	l        *myauth.Login
	clientID string
}

func (s setupLogin) URL() string            { return s.l.URL }
func (s setupLogin) Listening() bool        { return s.l.Listening }
func (s setupLogin) Paste(raw string) error { return s.l.Paste(raw) }
func (s setupLogin) Close()                 { s.l.Close() }
func (s setupLogin) Wait(ctx context.Context) error {
	token, err := s.l.Wait(ctx)
	if err != nil {
		return err
	}
	return saveLogin(s.clientID, token)
}

// saveLogin stores a new Spotify login: the token, the client ID it belongs
// to, and drops the library snapshot, which may be another account's.
func saveLogin(clientID string, token *oauth2.Token) error {
	if err := myauth.SaveToken(myauth.DefaultTokenPath(), token); err != nil {
		return err
	}
	_ = app.RemoveSnapshot(app.DefaultSnapshotPath())
	return config.Save(config.Config{ClientID: clientID})
}

// savedAccountDevices lists the Spotify devices of the account just saved.
func savedAccountDevices(ctx context.Context) ([]source.Device, error) {
	tokenPath := myauth.DefaultTokenPath()
	token, err := myauth.LoadToken(tokenPath)
	if err != nil {
		return nil, err
	}
	src := myspotify.NewPlayerSource(myspotify.NewClient(effectiveClientID(), token, tokenPath))
	return src.Devices(ctx)
}

// setupPlayer is the player package, as the setup sees it.
type setupPlayer struct{}

func (setupPlayer) Installed() bool {
	cfg, _ := config.Load()
	_, err := player.Find(cfg.PlayerPath)
	return err == nil
}

func (setupPlayer) Installer() (args []string, manager, command string) {
	in := player.FindInstaller()
	return in.Args, in.Manager, in.Command
}

func (setupPlayer) Login(ctx context.Context, paste bool, in io.Reader, out io.Writer) error {
	cfg, _ := config.Load()
	bin, err := player.Find(cfg.PlayerPath)
	if err != nil {
		return err
	}
	return player.Login(ctx, bin, player.Dir(), paste, in, out)
}

func (setupPlayer) Enable() error      { return config.Save(config.Config{Player: player.Backend}) }
func (setupPlayer) DeviceName() string { return player.DeviceName() }

// runSetup runs the guided setup full-screen and returns it, for the caller
// to check what got done.
func runSetup(deps app.SetupDeps, start app.SetupStart) *app.Setup {
	applyTheme()
	s := app.NewSetup(deps, start)
	_, err := tea.NewProgram(s, tea.WithAltScreen()).Run()
	s.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup: %v\n", err)
	}
	return s
}

// applyTheme applies the configured colour theme early, so the setup uses it.
func applyTheme() {
	cfg, _ := config.Load()
	overrides, _ := app.PaletteFromMap(cfg.Colors)
	if p, err := app.ResolveTheme(cfg.Theme, overrides); err == nil {
		app.ApplyTheme(p)
	}
}

// interactiveTerminal reports whether stdin and stdout are a terminal, so
// the full-screen setup can run.
func interactiveTerminal() bool {
	out := os.Stdout.Fd()
	return newOnboarding().interactive && (isatty.IsTerminal(out) || isatty.IsCygwinTerminal(out))
}

// runAuthSetup is `waxon auth` in a terminal: the guided setup, with the
// client ID decided by flags, $SPOTIFY_CLIENT_ID or a personal ID already
// configured (as chooseClientID does for the text flow).
func runAuthSetup(opts authOptions) int {
	saved, _ := config.Load()
	id, note := "", ""
	switch {
	case opts.clientID != "":
		var err error
		if id, err = myauth.NormalizeClientID(opts.clientID); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
	case opts.shared:
		id = myauth.DefaultClientID
	case opts.own:
	case os.Getenv("SPOTIFY_CLIENT_ID") != "":
		id = os.Getenv("SPOTIFY_CLIENT_ID")
		note = "Using the client ID in $SPOTIFY_CLIENT_ID."
	case saved.ClientID != "" && saved.ClientID != myauth.DefaultClientID:
		id = saved.ClientID
		note = "Using your Spotify app (" + short(id) + "). 'waxon auth --own' or '--shared' switches."
	}
	if s := runSetup(setupDeps(id, note), app.SetupFull); !s.Connected() {
		return 1
	}
	fmt.Println("You're all set! Run 'waxon' to start.")
	return 0
}
