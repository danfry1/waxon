package app

import (
	"context"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/auth"
	"github.com/danfry1/waxon/source"
)

// Setup is waxon's guided setup: connect a Spotify account, then choose where
// music plays — the user's own Spotify apps, or waxon's built-in player
// (librespot), which it can install and sign in. It runs on its own before
// the main UI (first launch, `waxon auth`, `waxon player login`) and inside
// it, from the device picker's "play here" row.
//
// Everything with side effects (browser sign-in, saving, installing) comes in
// through SetupDeps, which main provides.
type Setup struct {
	deps  SetupDeps
	start SetupStart
	ctx   context.Context
	stop  context.CancelFunc

	step          setupStep
	width, height int
	cursor        int // highlighted choice
	input         textinput.Model
	inputErr      string
	spin          spinner.Model
	err           string // the current step's failure, with what to do
	flash         string // brief confirmation ("Copied")

	clientID string
	login    SetupLogin
	pasting  bool // the paste field is open on the approve step

	devices       []source.Device
	devicesLoaded bool

	playerURL    string // librespot's sign-in link
	playerPaste  bool   // librespot is waiting for a pasted address
	playerIn     *io.PipeWriter
	playerEvents chan tea.Msg
	playerCancel context.CancelFunc

	// attempt numbers the current sign-in (Spotify's or the player's);
	// results of an earlier, abandoned one are dropped.
	attempt int

	connected bool // the Spotify account was saved
	playerOn  bool
	finished  bool
	quit      bool
}

// SetupLogin is one Spotify sign-in in progress (auth.Login, plus saving).
type SetupLogin interface {
	URL() string
	Listening() bool
	Paste(raw string) error
	// Wait completes the sign-in and saves the account.
	Wait(ctx context.Context) error
	Close()
}

// SetupPlayer is waxon's built-in player (the player package).
type SetupPlayer interface {
	Installed() bool
	// Installer is the package-manager command that installs librespot (nil
	// args: none found; command then says what to do by hand).
	Installer() (args []string, manager, command string)
	// Login signs librespot in, reading a pasted address from in and
	// writing its prompts (with the sign-in link) to out.
	Login(ctx context.Context, paste bool, in io.Reader, out io.Writer) error
	// Enable turns the player on in the config.
	Enable() error
	DeviceName() string
}

// SetupDeps are the setup's side effects.
type SetupDeps struct {
	// ClientID is an already-decided client ID (a flag, $SPOTIFY_CLIENT_ID
	// or a personal one in config), which skips the app choice. ClientIDNote
	// says where it came from.
	ClientID     string
	ClientIDNote string
	SharedID     string // the bundled shared client ID
	Remote       bool   // over SSH: the browser is on another machine

	RedirectURI  func(clientID string) string
	DashboardURL string
	BeginLogin   func(clientID string) (SetupLogin, error)
	// Devices lists the account's Spotify devices once signed in.
	Devices func(ctx context.Context) ([]source.Device, error)
	// Player is nil when the built-in player isn't offered.
	Player SetupPlayer

	OpenBrowser func(url string) error
	Copy        func(text string) error
}

// SetupStart picks where the setup begins.
type SetupStart int

const (
	// SetupFull connects Spotify, then asks where music plays.
	SetupFull SetupStart = iota
	// SetupPlayer installs (if needed) and signs in the built-in player.
	SetupPlayerOnly
)

type setupStep int

const (
	stepWelcome setupStep = iota
	stepChooseApp
	stepOwnApp
	stepApprove
	stepPlayback
	stepInstall
	stepPlayerLogin
	stepDone
)

// Messages the setup sends itself. setupMsg marks them so the main UI can
// route them to an embedded Setup.
type setupMsg interface{ isSetupMsg() }

type (
	setupLoginStartedMsg struct {
		login   SetupLogin
		err     error
		attempt int
	}
	setupLoginDoneMsg struct {
		err     error
		attempt int
	}
	setupDevicesMsg     struct{ devices []source.Device }
	setupInstallDoneMsg struct{ err error }
	setupPlayerURLMsg   struct {
		url     string
		attempt int
	}
	setupPlayerPasteMsg struct{ attempt int }
	setupPlayerDoneMsg  struct {
		err     error
		attempt int
	}
	setupFlashDoneMsg struct{}
)

func (setupLoginStartedMsg) isSetupMsg() {}
func (setupLoginDoneMsg) isSetupMsg()    {}
func (setupDevicesMsg) isSetupMsg()      {}
func (setupInstallDoneMsg) isSetupMsg()  {}
func (setupPlayerURLMsg) isSetupMsg()    {}
func (setupPlayerPasteMsg) isSetupMsg()  {}
func (setupPlayerDoneMsg) isSetupMsg()   {}
func (setupFlashDoneMsg) isSetupMsg()    {}

// NewSetup returns a setup starting at start.
func NewSetup(deps SetupDeps, start SetupStart) *Setup {
	ctx, stop := context.WithCancel(context.Background())
	ti := textinput.New()
	ti.CharLimit = 64
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	s := &Setup{deps: deps, start: start, ctx: ctx, stop: stop, input: ti, spin: sp, width: 80, height: 24}
	if start == SetupPlayerOnly {
		s.step = s.playerStep()
	}
	return s
}

// Finished reports whether the setup completed; PlayerOn whether it turned
// the built-in player on. Quit means the user left before finishing.
func (s *Setup) Finished() bool { return s.finished }

// Connected reports whether the Spotify account was saved (even if the user
// left before choosing where music plays).
func (s *Setup) Connected() bool { return s.connected }
func (s *Setup) PlayerOn() bool  { return s.playerOn }
func (s *Setup) Quit() bool      { return s.quit }

// Init starts what the first step needs.
func (s *Setup) Init() tea.Cmd { return s.enter() }

// Update is the tea.Model form of update, for running the setup on its own.
func (s *Setup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := s.update(msg)
	if s.finished || s.quit {
		return s, tea.Quit
	}
	return s, cmd
}

func (s *Setup) update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		return nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		s.spin, cmd = s.spin.Update(msg)
		return cmd
	case tea.KeyMsg:
		return s.handleKey(msg)
	case setupFlashDoneMsg:
		s.flash = ""
		return nil

	case setupLoginStartedMsg:
		if msg.attempt != s.attempt {
			if msg.login != nil {
				msg.login.Close() // abandoned before it started
			}
			return nil
		}
		if msg.err != nil {
			s.err = "Couldn't start the sign-in: " + msg.err.Error()
			return nil
		}
		s.login = msg.login
		s.pasting = s.deps.Remote || !msg.login.Listening()
		if s.pasting {
			s.focusInput("paste the address here", 0)
		}
		if !s.deps.Remote {
			s.openURL(msg.login.URL())
		}
		login, ctx, attempt := msg.login, s.ctx, s.attempt
		return tea.Batch(s.spin.Tick, func() tea.Msg { return setupLoginDoneMsg{login.Wait(ctx), attempt} })
	case setupLoginDoneMsg:
		if msg.attempt != s.attempt {
			return nil
		}
		s.login = nil
		if msg.err != nil {
			s.input.Blur()
			s.pasting = false
			s.err = "The sign-in didn't finish: " + msg.err.Error() + ". Press enter to try again."
			return nil
		}
		s.connected = true
		s.step = stepPlayback
		return s.enter()
	case setupDevicesMsg:
		s.devices, s.devicesLoaded = msg.devices, true
		return nil

	case setupInstallDoneMsg:
		if s.deps.Player.Installed() {
			s.step = stepPlayerLogin
			return s.enter()
		}
		_, manager, command := s.deps.Player.Installer()
		s.err = "librespot still isn't installed. Run '" + command + "' yourself, then press r to check again."
		if manager == "cargo" {
			s.err += " (cargo puts it in ~/.cargo/bin, which needs to be on your PATH.)"
		}
		return nil

	case setupPlayerURLMsg:
		if msg.attempt != s.attempt {
			return nil
		}
		s.playerURL = msg.url
		return s.nextPlayerEvent()
	case setupPlayerPasteMsg:
		if msg.attempt != s.attempt {
			return nil
		}
		s.playerPaste = true
		s.focusInput("paste the address here", 0)
		return s.nextPlayerEvent()
	case setupPlayerDoneMsg:
		if msg.attempt != s.attempt {
			return nil
		}
		s.playerCancel = nil
		if msg.err != nil {
			s.input.Blur()
			s.playerPaste = false
			s.err = "The player's sign-in didn't finish: " + msg.err.Error() + ". Press enter to try again."
			return nil
		}
		if err := s.deps.Player.Enable(); err != nil {
			s.err = "Signed in, but couldn't turn the player on: " + err.Error()
			return nil
		}
		s.playerOn = true
		s.step = stepDone
		return nil
	}
	return nil
}

// enter starts the current step's work.
func (s *Setup) enter() tea.Cmd {
	s.err, s.cursor = "", 0
	switch s.step {
	case stepOwnApp:
		s.focusInput("32 characters from the app's Settings", 64)
		if !s.deps.Remote {
			s.openURL(s.deps.DashboardURL)
		}
	case stepApprove:
		s.pasting = false
		s.attempt++
		clientID, begin, attempt := s.clientID, s.deps.BeginLogin, s.attempt
		return func() tea.Msg {
			l, err := begin(clientID)
			return setupLoginStartedMsg{l, err, attempt}
		}
	case stepPlayback:
		if s.deps.Player == nil {
			s.step = stepDone
			return nil
		}
		if s.deps.Devices == nil {
			s.devicesLoaded = true
			return nil
		}
		ctx, list := s.ctx, s.deps.Devices
		return tea.Batch(s.spin.Tick, func() tea.Msg {
			lookup, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			devices, _ := list(lookup) // the step works without the list
			return setupDevicesMsg{devices}
		})
	case stepPlayerLogin:
		return s.startPlayerLogin()
	}
	return nil
}

// playerStep is where setting up the player starts: install, or sign in.
func (s *Setup) playerStep() setupStep {
	if s.deps.Player != nil && s.deps.Player.Installed() {
		return stepPlayerLogin
	}
	return stepInstall
}

func (s *Setup) handleKey(msg tea.KeyMsg) tea.Cmd {
	if msg.Type == tea.KeyCtrlC {
		s.cancelWork()
		s.quit = true
		return nil
	}
	switch msg.String() {
	case "ctrl+o":
		s.openURL(s.currentURL())
		return nil
	case "ctrl+y":
		return s.copy(s.currentCopy())
	}

	if s.input.Focused() {
		return s.handleInputKey(msg)
	}

	// On the two choice steps, a number picks that option and continues.
	if k := msg.String(); (k == "1" || k == "2") && (s.step == stepChooseApp || s.step == stepPlayback) {
		s.cursor = int(k[0] - '1')
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	}

	switch s.step {
	case stepWelcome:
		switch msg.String() {
		case "enter":
			if s.deps.ClientID != "" {
				s.clientID = s.deps.ClientID
				s.step = stepApprove
			} else {
				s.step = stepChooseApp
			}
			return s.enter()
		case "q", "esc":
			s.quit = true
		}
	case stepChooseApp:
		switch msg.String() {
		case "j", "down", "k", "up", "tab":
			s.cursor = 1 - s.cursor
		case "enter":
			if s.cursor == 1 {
				s.clientID = s.deps.SharedID
				s.step = stepApprove
			} else {
				s.step = stepOwnApp
			}
			return s.enter()
		case "esc":
			s.step = stepWelcome
		}
	case stepApprove:
		switch msg.String() {
		case "p":
			if s.login != nil {
				s.pasting = true
				s.focusInput("paste the address here", 0)
			}
		case "enter":
			if s.err != "" {
				return s.enter() // try again
			}
		case "esc":
			s.cancelWork()
			s.restartCtx()
			s.step = s.backFromApprove()
			return s.enter()
		}
	case stepPlayback:
		switch msg.String() {
		case "j", "down", "k", "up", "tab":
			s.cursor = 1 - s.cursor
		case "s":
			s.step = stepDone
		case "enter":
			if s.cursor == 0 {
				s.step = stepDone
				return nil
			}
			s.step = s.playerStep()
			return s.enter()
		}
	case stepInstall:
		switch msg.String() {
		case "enter":
			args, _, _ := s.deps.Player.Installer()
			if len(args) == 0 {
				return nil
			}
			s.err = ""
			return tea.ExecProcess(exec.Command(args[0], args[1:]...), func(err error) tea.Msg { //nolint:gosec // a fixed package-manager command
				return setupInstallDoneMsg{err}
			})
		case "r":
			return func() tea.Msg { return setupInstallDoneMsg{} }
		case "esc":
			return s.leavePlayer()
		}
	case stepPlayerLogin:
		switch msg.String() {
		case "enter":
			if s.err != "" {
				return s.enter()
			}
		case "esc":
			return s.leavePlayer()
		}
	case stepDone:
		if msg.String() == "enter" || msg.String() == "esc" {
			s.finished = true
		}
	}
	return nil
}

func (s *Setup) handleInputKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		s.input.Blur()
		s.inputErr = ""
		switch s.step {
		case stepOwnApp:
			s.step = stepChooseApp
		case stepPlayerLogin:
			return s.leavePlayer()
		}
		// On the approve step, closing the field just hides it.
		s.pasting = false
		return nil
	case tea.KeyEnter:
		value := strings.TrimSpace(s.input.Value())
		switch s.step {
		case stepOwnApp:
			id, err := auth.NormalizeClientID(value)
			if err != nil {
				s.inputErr = err.Error()
				return nil
			}
			s.input.Blur()
			s.clientID = id
			s.step = stepApprove
			return s.enter()
		case stepApprove:
			if s.login == nil {
				return nil
			}
			if err := s.login.Paste(value); err != nil {
				s.inputErr = err.Error()
				return nil
			}
			s.input.Blur()
			s.inputErr = ""
		case stepPlayerLogin:
			if w := s.playerIn; w != nil {
				// A pipe write blocks until librespot reads it; keep it off
				// the UI loop.
				go func() { _, _ = io.WriteString(w, value+"\n") }()
			}
			s.input.Blur()
			s.playerPaste = false
		}
		return nil
	}
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	if s.step == stepOwnApp {
		s.inputErr = "" // re-checked live in the view
	}
	return cmd
}

// backFromApprove is where esc on the approve step returns to.
func (s *Setup) backFromApprove() setupStep {
	switch {
	case s.deps.ClientID != "":
		return stepWelcome
	case s.clientID == s.deps.SharedID:
		return stepChooseApp
	default:
		return stepOwnApp
	}
}

// leavePlayer abandons setting up the player: back to the playback choice
// in the full setup, or out of a player-only setup.
func (s *Setup) leavePlayer() tea.Cmd {
	s.cancelWork()
	s.restartCtx()
	s.input.Blur()
	s.err, s.playerPaste = "", false
	if s.start == SetupPlayerOnly {
		s.quit = true
		return nil
	}
	s.step = stepPlayback
	s.cursor = 1
	return nil
}

func (s *Setup) startPlayerLogin() tea.Cmd {
	s.playerURL, s.playerPaste = "", false
	s.attempt++
	ctx, cancel := context.WithCancel(s.ctx)
	s.playerCancel = cancel
	pr, pw := io.Pipe()
	s.playerIn = pw
	events := make(chan tea.Msg, 8)
	s.playerEvents = events
	attempt := s.attempt
	out := &librespotOutput{events: events, attempt: attempt}
	p, paste := s.deps.Player, s.deps.Remote
	go func() {
		err := p.Login(ctx, paste, pr, out)
		_ = pw.Close()
		_ = pr.Close()
		events <- setupPlayerDoneMsg{err, attempt}
		close(events)
	}()
	return tea.Batch(s.spin.Tick, s.nextPlayerEvent())
}

// nextPlayerEvent waits for the next thing librespot's sign-in reports.
func (s *Setup) nextPlayerEvent() tea.Cmd {
	events := s.playerEvents
	if events == nil {
		return nil
	}
	return func() tea.Msg {
		msg, ok := <-events
		if !ok {
			return nil
		}
		return msg
	}
}

// librespotOutput turns librespot's sign-in output into setup messages: the
// "Browse to:" link and its request for a pasted address.
type librespotOutput struct {
	events  chan<- tea.Msg
	attempt int
	partial string
}

func (o *librespotOutput) Write(p []byte) (int, error) {
	o.partial += string(p)
	for {
		line, rest, ok := strings.Cut(o.partial, "\n")
		if !ok {
			break
		}
		o.partial = rest
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "Browse to:"):
			o.events <- setupPlayerURLMsg{strings.TrimSpace(strings.TrimPrefix(line, "Browse to:")), o.attempt}
		case strings.Contains(line, "Provide redirect URL"):
			o.events <- setupPlayerPasteMsg{o.attempt}
		}
	}
	return len(p), nil
}

func (s *Setup) cancelWork() {
	s.attempt++ // whatever it was doing reports back to nobody
	if s.login != nil {
		s.login.Close()
		s.login = nil
	}
	if s.playerCancel != nil {
		s.playerCancel()
		s.playerCancel = nil
	}
	s.stop()
}

// restartCtx gives later steps a live context after cancelWork.
func (s *Setup) restartCtx() {
	s.ctx, s.stop = context.WithCancel(context.Background())
}

// Close cancels anything still running (the setup was left mid-way).
func (s *Setup) Close() { s.cancelWork() }

// focusInput opens the text field; limit 0 means no length limit (pasted
// addresses run to a few hundred characters).
func (s *Setup) focusInput(placeholder string, limit int) {
	s.input.Reset()
	s.input.CharLimit = limit
	s.input.Placeholder = placeholder
	s.inputErr = ""
	s.input.Focus()
}

// currentURL is what ctrl+o opens on this step.
func (s *Setup) currentURL() string {
	switch s.step {
	case stepOwnApp:
		return s.deps.DashboardURL
	case stepApprove:
		if s.login != nil {
			return s.login.URL()
		}
	case stepPlayerLogin:
		return s.playerURL
	}
	return ""
}

// currentCopy is what ctrl+y copies on this step.
func (s *Setup) currentCopy() string {
	if s.step == stepOwnApp && s.deps.RedirectURI != nil {
		return s.deps.RedirectURI(s.clientID)
	}
	return s.currentURL()
}

func (s *Setup) openURL(url string) {
	if url == "" || s.deps.OpenBrowser == nil {
		return
	}
	_ = s.deps.OpenBrowser(url)
}

func (s *Setup) copy(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	copyFn := s.deps.Copy
	if copyFn == nil {
		copyFn = copyToClipboard
	}
	if err := copyFn(text); err != nil {
		s.flash = "Couldn't copy: " + err.Error()
	} else {
		s.flash = "Copied"
	}
	return tea.Tick(1500*time.Millisecond, func(time.Time) tea.Msg { return setupFlashDoneMsg{} })
}
