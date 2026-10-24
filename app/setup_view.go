package app

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/danfry1/waxon/auth"
)

// View draws the current setup step as a centered card.
func (s *Setup) View() string {
	w := min(66, s.width-4)
	if w < 30 || s.height < 12 {
		return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center,
			StyleDimText.Render("Make the terminal a little bigger to continue setup"))
	}
	inner := w - 6 // border + padding

	title, body, keys := s.stepContent(inner)
	var b strings.Builder
	if p := s.progress(); p != "" {
		b.WriteString(p + "\n\n")
	}
	b.WriteString(setupTitle.Render(title) + "\n\n")
	b.WriteString(body)
	if s.err != "" {
		b.WriteString("\n\n" + setupError.Width(inner).Render(s.err))
	}
	if s.flash != "" {
		b.WriteString("\n\n" + setupGood.Render("✓ "+s.flash))
	}
	b.WriteString("\n\n" + StyleDimText.Width(inner).Render(keys))

	card := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Padding(1, 2).
		Width(w).
		Render(b.String())
	return lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, card)
}

var (
	setupTitle = lipgloss.NewStyle().Bold(true)
	setupError = lipgloss.NewStyle()
	setupGood  = lipgloss.NewStyle()
)

// restyleSetup picks up the active theme's colours.
func restyleSetup() {
	setupTitle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	setupError = lipgloss.NewStyle().Foreground(ColorError)
	setupGood = lipgloss.NewStyle().Foreground(ColorAccent)
}

// progress is the "Connect · Playback · Ready" trail for the full setup.
func (s *Setup) progress() string {
	if s.start != SetupFull || s.step == stepWelcome {
		return ""
	}
	stage := 0
	switch s.step {
	case stepPlayback, stepInstall, stepPlayerLogin:
		stage = 1
	case stepDone:
		stage = 2
	}
	names := []string{"Connect Spotify", "Playback", "Ready"}
	if s.deps.Player == nil {
		names = []string{"Connect Spotify", "Ready"}
		if stage > 0 {
			stage = 1
		}
	}
	parts := make([]string, len(names))
	for i, n := range names {
		switch {
		case i < stage:
			parts[i] = setupGood.Render("✓ " + n)
		case i == stage:
			parts[i] = lipgloss.NewStyle().Foreground(ColorText).Bold(true).Render("● " + n)
		default:
			parts[i] = StyleDimText.Render("○ " + n)
		}
	}
	return strings.Join(parts, StyleDimText.Render("  ·  "))
}

func (s *Setup) stepContent(width int) (title, body, keys string) {
	text := lipgloss.NewStyle().Foreground(ColorText).Width(width)
	link := lipgloss.NewStyle().Foreground(ColorAccent)
	dim := StyleDimText.Width(width)
	switch s.step {
	case stepWelcome:
		return "Welcome to waxon",
			text.Render("A vim-style Spotify player for your terminal.") + "\n\n" +
				dim.Render("Setup takes about a minute: connect your Spotify account, then choose where music plays. Controlling playback needs Spotify Premium."),
			"enter start · q quit"

	case stepChooseApp:
		return "How should waxon connect?",
			dim.Render("Spotify needs a developer \"app\" (a free API key) before anything can talk to it.") + "\n\n" +
				s.card(0, "Your own app", "Recommended",
					"About 2 minutes: waxon shows every click. No rate limits, so no pauses.", width) + "\n" +
				s.card(1, "waxon's shared app", "Zero setup",
					"Works straight away, but Spotify throttles it for every waxon user at once, so expect occasional pauses.", width) + "\n\n" +
				dim.Render("Not sure? Pick your own app. You can switch later with waxon auth."),
			"1 / 2 or ↑↓ choose · enter continue · esc back"

	case stepOwnApp:
		redirect := ""
		if s.deps.RedirectURI != nil {
			redirect = s.deps.RedirectURI("personal")
		}
		steps := []string{
			"Open the Spotify dashboard and log in" + s.openedNote(),
			"Click Create app. Name and description: anything, e.g. waxon",
			"Redirect URI: paste " + lipgloss.NewStyle().Foreground(ColorAccent).Render(redirect) + " and click Add",
			"Tick Web API, then Save",
			"Open the app's Settings and copy its Client ID",
		}
		caveat := "Spotify blocks one thing for personal apps: an artist's top tracks (artist pages still list their albums)."
		var b strings.Builder
		num := lipgloss.NewStyle().Foreground(ColorAccent).Width(3)
		item := lipgloss.NewStyle().Foreground(ColorText).Width(width - 3)
		for i, st := range steps {
			b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, num.Render(strconv.Itoa(i+1)+"."), item.Render(st)) + "\n")
		}
		b.WriteString("\n" + s.inputLine("Client ID", s.clientIDStatus(), width))
		b.WriteString("\n\n" + dim.Render(caveat))
		return "Create your Spotify app", b.String(),
			"enter continue · ctrl+o open dashboard · ctrl+y copy redirect URI · esc back"

	case stepApprove:
		var b strings.Builder
		switch {
		case s.login == nil && s.err == "":
			b.WriteString(s.spin.View() + " Starting…")
		case s.login == nil:
		case s.deps.Remote:
			b.WriteString(text.Render("Open this link in a browser on any device and approve waxon:") + "\n\n")
			b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Width(width).Render(s.login.URL()) + "\n\n")
			b.WriteString(dim.Render("The page after approving won't load (it can't reach this machine). That's expected: copy its address from the address bar.") + "\n\n")
		default:
			b.WriteString(s.spin.View() + " " + text.Render("Approve waxon in your browser") + "\n\n")
			b.WriteString(dim.Render("Waiting for Spotify. Browser didn't open? Open this link (ctrl+y copies it):") + "\n\n")
			b.WriteString(link.Width(width).Render(s.login.URL()))
			if !s.login.Listening() {
				b.WriteString("\n\n" + dim.Render("Another program is using waxon's sign-in port, so the page after approving won't load. Copy its address from the address bar and paste it below."))
			}
		}
		if s.deps.ClientIDNote != "" {
			b.WriteString("\n\n" + dim.Render(s.deps.ClientIDNote))
		}
		if s.pasting && s.login != nil {
			b.WriteString("\n\n" + s.inputLine("Address", s.inputErr, width))
		}
		keys := "ctrl+o open link · ctrl+y copy link · esc back"
		if !s.pasting && s.login != nil {
			keys = "p paste the address instead · " + keys
		}
		return "Sign in to Spotify", b.String(), keys

	case stepPlayback:
		var b strings.Builder
		switch {
		case !s.devicesLoaded:
			b.WriteString(dim.Render(s.spin.View()+" Checking your Spotify devices…") + "\n\n")
		case len(s.devices) == 0:
			b.WriteString(dim.Render("No Spotify apps are open on your account right now.") + "\n\n")
		default:
			names := make([]string, len(s.devices))
			for i, d := range s.devices {
				names[i] = d.Name
			}
			found := StyleDimText.Render("Found on your account: ") + lipgloss.NewStyle().Foreground(ColorText).Render(strings.Join(names, " · "))
			b.WriteString(lipgloss.NewStyle().Width(width).Render(found) + "\n\n")
		}
		b.WriteString(s.card(0, "Your Spotify apps", "Nothing to install",
			"Your phone, the desktop app or a speaker plays; waxon is the remote.", width) + "\n")
		b.WriteString(s.card(1, "Right here in waxon", s.playerStatus(),
			"No Spotify app needed. Uses librespot, an open-source player that signs in as the Spotify desktop app (unofficial); its login stays on this computer.", width) + "\n\n")
		b.WriteString(dim.Render("Not sure? Start with your Spotify apps. Press D in waxon to add this any time."))
		return "Where should music play?", b.String(), "1 / 2 or ↑↓ choose · enter continue"

	case stepInstall:
		args, manager, command := s.deps.Player.Installer()
		cmd := lipgloss.NewStyle().Foreground(ColorAccent).Render(command)
		if len(args) == 0 {
			return "Install librespot",
				text.Render("waxon's player runs librespot, which isn't installed. No package manager that has it was found, so:") + "\n\n" + cmd,
				"r check again · esc back"
		}
		return "Install librespot",
			text.Render("waxon's player runs librespot, which isn't installed yet. Install it with "+manager+"?") + "\n\n  " + cmd + "\n\n" +
				dim.Render("It runs in this terminal, so you'll see its output (and any password prompt). waxon comes back when it's done."),
			"enter install · r check again · esc back"

	case stepPlayerLogin:
		var b strings.Builder
		b.WriteString(dim.Render("Spotify only lets its own apps play audio, so the player signs in separately. Approve it in your browser (it's Spotify's own sign-in page).") + "\n\n")
		switch {
		case s.err != "":
		case s.deps.Remote && s.playerURL != "":
			b.WriteString(text.Render("Open this link on any device:") + "\n\n" + link.Width(width).Render(s.playerURL))
		default:
			b.WriteString(s.spin.View() + " " + text.Inline(true).Render("Waiting for approval…"))
			if s.playerURL != "" {
				b.WriteString("\n\n" + dim.Render("Browser didn't open? Open this link (ctrl+y copies it):") + "\n\n" + link.Width(width).Render(s.playerURL))
			}
		}
		if s.playerPaste {
			b.WriteString("\n\n" + dim.Render("After approving, the page won't load. That's expected: copy its address from the address bar and paste it here.") +
				"\n\n" + s.inputLine("Address", "", width))
		}
		return "Sign in the player", b.String(), "ctrl+o open link · ctrl+y copy link · esc back"

	case stepDone:
		var b strings.Builder
		if s.start == SetupFull {
			b.WriteString(setupGood.Render("✓") + " " + text.Inline(true).Render("Connected to Spotify") + "\n")
		}
		if s.playerOn {
			b.WriteString(setupGood.Render("✓") + " " + text.Inline(true).Render("Music plays right here, as \""+s.deps.Player.DeviceName()+"\"") + "\n")
		} else {
			b.WriteString(setupGood.Render("✓") + " " + text.Inline(true).Render("Music plays on your Spotify apps") + "\n")
			if s.deps.Player != nil {
				b.WriteString(dim.Render("   Want to play right here later? Press D in waxon.") + "\n")
			}
		}
		b.WriteString("\n" + dim.Render("A few keys to start with:") + "\n")
		for _, tip := range [][2]string{{"?", "every key"}, {"enter", "play the selected track"}, {"N", "now playing (l lyrics, V vinyl)"}, {"D", "choose a device"}} {
			b.WriteString("  " + lipgloss.NewStyle().Foreground(ColorAccent).Width(7).Render(tip[0]) + text.Inline(true).Render(tip[1]) + "\n")
		}
		return "You're all set", strings.TrimRight(b.String(), "\n"), "enter start"
	}
	return "", "", ""
}

// card renders option i as a bordered box: its number key, name and badge,
// then a description. The highlighted one gets the accent border.
func (s *Setup) card(i int, name, badge, desc string, width int) string {
	selected := s.cursor == i
	border, numStyle := ColorBorder, StyleDimText
	nameStyle := lipgloss.NewStyle().Foreground(ColorTextSec).Bold(true)
	badgeStyle := StyleDimText
	descStyle := lipgloss.NewStyle().Foreground(ColorTextDim)
	if selected {
		border = ColorAccent
		numStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
		nameStyle = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
		badgeStyle = lipgloss.NewStyle().Foreground(ColorAccent)
		descStyle = lipgloss.NewStyle().Foreground(ColorText)
	}
	inner := width - 4 // border and padding
	head := numStyle.Render(strconv.Itoa(i+1)) + "  " + nameStyle.Render(name)
	tag := badgeStyle.Render(badge)
	if gap := inner - lipgloss.Width(head) - lipgloss.Width(tag); gap >= 2 {
		head += strings.Repeat(" ", gap) + tag
	} else {
		head += "\n   " + tag
	}
	body := descStyle.Width(inner - 3).PaddingLeft(3).Render(desc)
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1).
		Width(width - 2).
		Render(head + "\n" + body)
}

func (s *Setup) inputLine(label, status string, width int) string {
	line := StyleDimText.Render(label+": ") + s.input.View()
	if status != "" {
		line += "\n" + lipgloss.NewStyle().Width(width).Render(status)
	}
	return line
}

// clientIDStatus checks the client ID as it's typed.
func (s *Setup) clientIDStatus() string {
	v := strings.TrimSpace(s.input.Value())
	if v == "" {
		return ""
	}
	if _, err := auth.NormalizeClientID(v); err != nil {
		return StyleDimText.Render(err.Error())
	}
	return setupGood.Render("✓ looks right — press enter")
}

func (s *Setup) openedNote() string {
	if s.deps.Remote {
		return " (" + s.deps.DashboardURL + ")"
	}
	return " (opened in your browser)"
}

// playerStatus is the built-in player option's badge.
func (s *Setup) playerStatus() string {
	if s.deps.Player.Installed() {
		return "librespot ready"
	}
	return "Installs librespot"
}
