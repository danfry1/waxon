package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// updateMsg carries what Options.CheckUpdate found: a newer release's
// version, or "".
type updateMsg struct{ version string }

func (m Model) checkUpdate() tea.Cmd {
	check := m.opts.CheckUpdate
	if check == nil {
		return nil
	}
	return func() tea.Msg { return updateMsg{check()} }
}

func (m Model) handleUpdate(msg updateMsg) (Model, tea.Cmd) {
	if msg.version == "" {
		return m, nil
	}
	m.newVersion = msg.version
	// A toast already up (the player didn't start, say) matters more: wait
	// for it to go.
	if m.toast.Visible() {
		v := msg.version
		return m, tea.Tick(toastDuration, func(time.Time) tea.Msg { return updateMsg{v} })
	}
	if m.opts.Settings.UpdateCheckOff {
		return m, nil
	}
	m.toast.Show("waxon "+msg.version+" is out", m.upgradeHint(), ToastInfo)
	return m, scheduleAutoDismiss()
}

// upgradeHint says how to get the newer release.
func (m Model) upgradeHint() string {
	if m.opts.UpgradeCommand != "" {
		return "Upgrade: " + m.opts.UpgradeCommand
	}
	return "github.com/danfry1/waxon/releases"
}

// NewVersion is a newer release found at launch, or "" (also once the
// settings turned the check off): waxon says so again as it quits, where
// the command can be copied.
func (m Model) NewVersion() string {
	if m.opts.Settings.UpdateCheckOff {
		return ""
	}
	return m.newVersion
}
