package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

const sharedTestID = "0123456789abcdef0123456789abcdef"

type fakeLogin struct {
	listening bool
	pasted    chan string
	waitErr   error
}

func (f *fakeLogin) URL() string     { return "https://accounts.spotify.com/authorize?x" }
func (f *fakeLogin) Listening() bool { return f.listening }
func (f *fakeLogin) Close()          {}
func (f *fakeLogin) Paste(raw string) error {
	if !strings.Contains(raw, "code=") {
		return errors.New("that doesn't look like the redirect address")
	}
	f.pasted <- raw
	return nil
}

func (f *fakeLogin) Wait(ctx context.Context) error {
	if f.listening {
		return f.waitErr // the browser redirect arrives at once
	}
	select {
	case <-f.pasted:
		return f.waitErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

type fakePlayer struct {
	mu        sync.Mutex
	installed bool
	loggedIn  bool
	enabled   bool
	loginErr  error
	paste     bool // ask for a pasted address, like librespot over SSH
	pasted    string
}

func (p *fakePlayer) Installed() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.installed
}

func (p *fakePlayer) LoggedIn() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.loggedIn
}

func (p *fakePlayer) Installer() ([]string, string, string) {
	return []string{"brew", "install", "librespot"}, "Homebrew", "brew install librespot"
}

func (p *fakePlayer) Login(_ context.Context, paste bool, in io.Reader, out io.Writer) error {
	fmt.Fprintln(out, "[INFO  librespot] starting")
	fmt.Fprintln(out, "Browse to: https://accounts.spotify.com/authorize?player")
	if p.paste {
		fmt.Fprintln(out, "Provide redirect URL")
		line, _ := readLine(in)
		p.mu.Lock()
		p.pasted = line
		p.mu.Unlock()
	}
	return p.loginErr
}
func (p *fakePlayer) Enable() error      { p.enabled = true; return nil }
func (p *fakePlayer) DeviceName() string { return "waxon (box)" }

func readLine(r io.Reader) (string, error) {
	var b []byte
	buf := make([]byte, 1)
	for {
		n, err := r.Read(buf)
		if n > 0 {
			if buf[0] == '\n' {
				return string(b), nil
			}
			b = append(b, buf[0])
		}
		if err != nil {
			return string(b), err
		}
	}
}

type setupHarness struct {
	t       *testing.T
	s       *Setup
	begun   []string
	login   *fakeLogin
	player  *fakePlayer
	devices []source.Device
	msgs    chan tea.Msg
}

func newSetupHarness(t *testing.T, start SetupStart, mutate func(*SetupDeps)) *setupHarness {
	h := &setupHarness{
		t:      t,
		login:  &fakeLogin{listening: true, pasted: make(chan string, 1)},
		player: &fakePlayer{},
		msgs:   make(chan tea.Msg, 64),
	}
	deps := SetupDeps{
		SharedID:     sharedTestID,
		RedirectURI:  func(string) string { return "http://127.0.0.1:8888/callback" },
		DashboardURL: "https://developer.spotify.com/dashboard",
		BeginLogin: func(id string) (SetupLogin, error) {
			h.begun = append(h.begun, id)
			return h.login, nil
		},
		Devices:     func(context.Context) ([]source.Device, error) { return h.devices, nil },
		Player:      h.player,
		OpenBrowser: func(string) error { return nil },
		Copy:        func(string) error { return nil },
	}
	if mutate != nil {
		mutate(&deps)
	}
	h.s = NewSetup(deps, start)
	h.dispatch(h.s.Init())
	h.settle()
	return h
}

// dispatch runs cmd in the background, as Bubbletea does; its message is
// handled by settle.
func (h *setupHarness) dispatch(cmd tea.Cmd) {
	if cmd != nil {
		go func() { h.msgs <- cmd() }()
	}
}

func (h *setupHarness) handle(msg tea.Msg) {
	switch msg := msg.(type) {
	case nil, spinner.TickMsg, setupFlashDoneMsg: // animation and timers
	case tea.BatchMsg:
		for _, c := range msg {
			h.dispatch(c)
		}
	default:
		h.dispatch(h.s.update(msg))
	}
}

// settle handles messages until none arrive for a while (commands still
// blocked, such as a sign-in waiting for a paste, are left running).
func (h *setupHarness) settle() {
	for {
		select {
		case msg := <-h.msgs:
			h.handle(msg)
		case <-time.After(300 * time.Millisecond):
			return
		}
	}
}

func (h *setupHarness) key(keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		h.handle(msg)
		h.settle()
	}
}

func (h *setupHarness) wantStep(want setupStep) {
	h.t.Helper()
	if h.s.step != want {
		h.t.Fatalf("step = %d, want %d (err %q)\n%s", h.s.step, want, h.s.err, h.s.View())
	}
}

func TestSetupSharedAppToYourDevices(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.devices = []source.Device{{Name: "Phone"}, {Name: "Laptop"}}
	h.key("enter") // welcome
	h.wantStep(stepChooseApp)
	h.key("2") // the shared app
	h.wantStep(stepPlayback)
	if len(h.begun) != 1 || h.begun[0] != sharedTestID {
		t.Fatalf("sign-in started with %v, want the shared ID", h.begun)
	}
	if v := h.s.View(); !strings.Contains(v, "Phone · Laptop") {
		t.Errorf("playback step should list the account's devices:\n%s", v)
	}
	h.key("enter") // your Spotify apps
	h.wantStep(stepDone)
	h.key("enter")
	if !h.s.Finished() || h.s.PlayerOn() || h.player.enabled {
		t.Errorf("finished=%v playerOn=%v enabled=%v; want finished with the player left off", h.s.Finished(), h.s.PlayerOn(), h.player.enabled)
	}
}

func TestSetupOwnAppChecksClientID(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.key("enter", "enter") // welcome, own app
	h.wantStep(stepOwnApp)
	h.key("abc", "enter")
	h.wantStep(stepOwnApp)
	if !strings.Contains(h.s.inputErr, "32 characters") {
		t.Errorf("short client ID error = %q", h.s.inputErr)
	}
	id := "AABBCCDDEEFF00112233445566778899"
	h.s.input.SetValue(id)
	h.key("enter")
	h.wantStep(stepPlayback)
	if len(h.begun) != 1 || h.begun[0] != strings.ToLower(id) {
		t.Errorf("sign-in started with %v, want the pasted ID lower-cased", h.begun)
	}
}

func TestSetupConfiguredClientIDSkipsChoice(t *testing.T) {
	h := newSetupHarness(t, SetupFull, func(d *SetupDeps) { d.ClientID = "ffffffffffffffffffffffffffffffff" })
	h.key("enter")
	h.wantStep(stepPlayback)
	if h.begun[0] != "ffffffffffffffffffffffffffffffff" {
		t.Errorf("sign-in started with %v", h.begun)
	}
}

func TestSetupPasteOverSSH(t *testing.T) {
	h := newSetupHarness(t, SetupFull, func(d *SetupDeps) { d.Remote = true })
	h.login.listening = false
	h.key("enter", "j", "enter")
	h.wantStep(stepApprove)
	if !h.s.pasting || !h.s.input.Focused() {
		t.Fatal("over SSH the paste field should open at once")
	}
	h.s.input.SetValue("not a url")
	h.key("enter")
	if h.s.inputErr == "" {
		t.Error("a bad paste should say what's wrong")
	}
	h.s.input.SetValue("http://127.0.0.1:8888/login?code=abc&state=s")
	h.key("enter") // the paste completes the waiting sign-in
	h.wantStep(stepPlayback)
}

func TestSetupFailedSignInCanRetry(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.login.waitErr = errors.New("auth denied: access_denied")
	h.key("enter", "j", "enter")
	h.wantStep(stepApprove)
	if !strings.Contains(h.s.err, "access_denied") {
		t.Fatalf("err = %q", h.s.err)
	}
	h.login.waitErr = nil
	h.key("enter")
	h.wantStep(stepPlayback)
	if len(h.begun) != 2 {
		t.Errorf("enter should start a new sign-in, began %d", len(h.begun))
	}
}

func TestSetupInstallsAndSignsInPlayer(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.key("enter", "j", "enter") // to playback
	h.key("j", "enter")          // right here in waxon
	h.wantStep(stepInstall)
	if v := h.s.View(); !strings.Contains(v, "brew install librespot") {
		t.Errorf("install step should show the command:\n%s", v)
	}
	// The package manager ran (tea.ExecProcess) and installed it.
	h.player.installed = true
	h.handle(setupInstallDoneMsg{})
	h.settle()
	h.wantStep(stepDone)
	if !h.player.enabled || !h.s.PlayerOn() {
		t.Error("a signed-in player should be turned on")
	}
	if h.s.playerURL != "https://accounts.spotify.com/authorize?player" {
		t.Errorf("player sign-in link = %q", h.s.playerURL)
	}
	if v := h.s.View(); !strings.Contains(v, `as "waxon (box)"`) {
		t.Errorf("done step should name the player's device:\n%s", v)
	}
}

func TestSetupInstallThatDidNotWork(t *testing.T) {
	h := newSetupHarness(t, SetupPlayerOnly, nil)
	h.wantStep(stepInstall)
	h.handle(setupInstallDoneMsg{err: errors.New("exit status 1")})
	h.settle()
	h.wantStep(stepInstall)
	if !strings.Contains(h.s.err, "brew install librespot") {
		t.Errorf("err = %q; should say how to install by hand", h.s.err)
	}
}

func TestSetupPlayerPasteOverSSH(t *testing.T) {
	player := &fakePlayer{installed: true, paste: true}
	h := newSetupHarness(t, SetupPlayerOnly, func(d *SetupDeps) { d.Remote = true; d.Player = player })
	h.player = player
	if !h.s.playerPaste || !h.s.input.Focused() {
		t.Fatalf("librespot's request for the address should open the paste field\n%s", h.s.View())
	}
	h.s.input.SetValue("http://127.0.0.1/login?code=xyz")
	h.key("enter")
	h.wantStep(stepDone)
	player.mu.Lock()
	defer player.mu.Unlock()
	if player.pasted != "http://127.0.0.1/login?code=xyz" {
		t.Errorf("librespot got %q", player.pasted)
	}
}

func TestSetupPlayerOnlyEscQuits(t *testing.T) {
	h := newSetupHarness(t, SetupPlayerOnly, nil)
	h.key("esc")
	if !h.s.Quit() || h.s.Finished() {
		t.Error("esc in a player-only setup should leave it")
	}
}

func TestSetupWithoutPlayerSkipsPlayback(t *testing.T) {
	h := newSetupHarness(t, SetupFull, func(d *SetupDeps) { d.Player = nil })
	h.key("enter", "j", "enter")
	h.wantStep(stepDone)
}

func TestDevicePickerOffersPlayHere(t *testing.T) {
	started := false
	h := newSetupHarness(t, SetupPlayerOnly, nil)
	h.player.installed = true
	deps := h.s.deps
	m := newTestModel(&StubSource{}).WithOptions(Options{
		PlayerSetup: &deps,
		StartPlayer: func() string { started = true; return "waxon (box)" },
	})
	res, _ := m.Update(devicesLoadedMsg{devices: []source.Device{{ID: "d1", Name: "Phone"}}})
	m = res.(Model)
	if m.devices == nil || len(m.devices.devices) != 2 || m.devices.devices[1].ID != playHereDeviceID {
		t.Fatalf("picker should end with the play-here row, got %+v", m.devices)
	}
	m.devices.MoveDown()
	res, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = res.(Model)
	if m.mode != ModeSetup || m.setup == nil {
		t.Fatalf("choosing it should open the player setup, mode=%v", m.mode)
	}
	// Run the setup to completion through the app: the player is installed,
	// so it signs in and finishes; enter leaves the done step.
	m = pumpModel(m, cmd)
	res, cmd = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = pumpModel(res.(Model), cmd)
	if m.mode != ModeNormal || !started || m.opts.PlayerName != "waxon (box)" {
		t.Errorf("after setup: mode=%v started=%v player=%q", m.mode, started, m.opts.PlayerName)
	}
	if m.canSetUpPlayer() {
		t.Error("with the player on, the play-here row should go away")
	}
}

// pumpModel runs cmd and what follows from it through m until it settles.
func pumpModel(m Model, cmd tea.Cmd) Model {
	msgs := make(chan tea.Msg, 64)
	dispatch := func(c tea.Cmd) {
		if c != nil {
			go func() { msgs <- c() }()
		}
	}
	dispatch(cmd)
	for {
		select {
		case msg := <-msgs:
			switch msg := msg.(type) {
			case nil, spinner.TickMsg, setupFlashDoneMsg, clearToastMsg:
			case tea.BatchMsg:
				for _, c := range msg {
					dispatch(c)
				}
			default:
				res, c := m.Update(msg)
				m = res.(Model)
				dispatch(c)
			}
		case <-time.After(300 * time.Millisecond):
			return m
		}
	}
}

func TestNoDevicesOpensPlayHerePicker(t *testing.T) {
	deps := SetupDeps{Player: &fakePlayer{}}
	m := newTestModel(&StubSource{}).WithOptions(Options{PlayerSetup: &deps})
	res, _ := m.Update(devicesLoadedMsg{})
	m = res.(Model)
	if m.mode != ModeDevices || len(m.devices.devices) != 1 {
		t.Fatalf("no devices: want the picker with just the play-here row, mode=%v", m.mode)
	}
	if !strings.Contains(m.devices.View(), "Play right here?") {
		t.Error("the picker should say why it opened")
	}
}

func TestSetupStaleSignInIgnoredAfterEsc(t *testing.T) {
	h := newSetupHarness(t, SetupFull, func(d *SetupDeps) { d.Remote = true })
	h.login.listening = false // waits for a paste, so it's still running
	h.key("enter", "j", "enter")
	h.wantStep(stepApprove)
	old := h.s.attempt
	h.key("esc") // close the paste field
	h.key("esc") // leave the step
	h.wantStep(stepChooseApp)
	// The abandoned attempt reports back late.
	h.handle(setupLoginDoneMsg{err: context.Canceled, attempt: old})
	h.handle(setupLoginStartedMsg{login: h.login, attempt: old})
	h.settle()
	h.wantStep(stepChooseApp)
	if h.s.err != "" || h.s.login != nil {
		t.Errorf("stale results changed the screen: err=%q login=%v", h.s.err, h.s.login)
	}
}

func TestSetupRetryWorksWithPasteFieldOpen(t *testing.T) {
	h := newSetupHarness(t, SetupFull, func(d *SetupDeps) { d.Remote = true })
	h.login.listening = false
	h.key("enter", "j", "enter")
	if !h.s.input.Focused() {
		t.Fatal("the paste field should be open")
	}
	// The sign-in fails (timed out, say) while the field is still open.
	h.handle(setupLoginDoneMsg{err: errors.New("timed out"), attempt: h.s.attempt})
	h.settle()
	if h.s.err == "" || h.s.input.Focused() {
		t.Fatalf("the error should show and the field close; err=%q focused=%v", h.s.err, h.s.input.Focused())
	}
	h.key("enter") // "Press enter to try again"
	if len(h.begun) != 2 {
		t.Errorf("enter should retry; sign-ins begun: %d", len(h.begun))
	}
}

func TestSetupPasteFieldTakesLongAddresses(t *testing.T) {
	h := newSetupHarness(t, SetupFull, func(d *SetupDeps) { d.Remote = true })
	h.login.listening = false
	h.key("enter", "j", "enter")
	long := "http://127.0.0.1:8888/callback?code=" + strings.Repeat("c", 200) + "&state=" + strings.Repeat("s", 86)
	h.s.input.SetValue(long)
	if got := h.s.input.Value(); got != long {
		t.Errorf("paste field cut the address to %d of %d characters", len(got), len(long))
	}
}

func TestSetupLeavingPlayerClearsError(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.key("enter", "j", "enter", "j", "enter")
	h.wantStep(stepInstall)
	h.handle(setupInstallDoneMsg{err: errors.New("exit status 1")})
	h.settle()
	h.key("esc")
	h.wantStep(stepPlayback)
	if h.s.err != "" {
		t.Errorf("the install error followed back to the playback step: %q", h.s.err)
	}
}

func TestSetupConnectedEvenIfLeftAtPlayback(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.key("enter", "j", "enter")
	h.wantStep(stepPlayback)
	h.s.update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if !h.s.Connected() || h.s.Finished() {
		t.Errorf("connected=%v finished=%v; the account was saved before quitting", h.s.Connected(), h.s.Finished())
	}
}

func TestSetupNumberKeysPickAndContinue(t *testing.T) {
	h := newSetupHarness(t, SetupFull, nil)
	h.key("enter")
	h.key("2") // the shared app, straight to sign-in
	h.wantStep(stepPlayback)
	if h.begun[0] != sharedTestID {
		t.Errorf("2 should pick the shared app, began %v", h.begun)
	}
	h.key("1") // your Spotify apps
	h.wantStep(stepDone)
}
