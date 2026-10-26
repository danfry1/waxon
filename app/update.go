package app

import tea "github.com/charmbracelet/bubbletea"

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

// NewVersion is a newer release found at launch, or "": waxon says so again
// as it quits, where the command can be copied.
func (m Model) NewVersion() string { return m.newVersion }
