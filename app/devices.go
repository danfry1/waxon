package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/danfry1/waxon/source"
)

// DevicePicker is a floating overlay listing available Spotify Connect devices.
type DevicePicker struct {
	devices []source.Device
	cursor  int
	width   int
	height  int
}

// NewDevicePicker creates a device picker popup from a list of devices.
func NewDevicePicker(devices []source.Device, width, height int) DevicePicker {
	return DevicePicker{
		devices: devices,
		cursor:  activeDeviceIndex(devices),
		width:   width,
		height:  height,
	}
}

// activeDeviceIndex returns the index of the active device, or 0 if none.
func activeDeviceIndex(devices []source.Device) int {
	for i, d := range devices {
		if d.IsActive {
			return i
		}
	}
	return 0
}

// MoveDown moves the cursor down.
func (d *DevicePicker) MoveDown() {
	d.cursor++
	if d.cursor >= len(d.devices) {
		d.cursor = len(d.devices) - 1
	}
}

// MoveUp moves the cursor up.
func (d *DevicePicker) MoveUp() {
	d.cursor--
	if d.cursor < 0 {
		d.cursor = 0
	}
}

// Selected returns the currently highlighted device, or nil if empty.
func (d DevicePicker) Selected() *source.Device {
	if d.cursor >= 0 && d.cursor < len(d.devices) {
		return &d.devices[d.cursor]
	}
	return nil
}

// View renders the device picker as a centered floating overlay.
func (d DevicePicker) View() string {
	overlayW := min(50, d.width-8)
	overlayH := min(len(d.devices)+6, d.height-4)

	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAccent).
		Background(ColorBg).
		Width(overlayW).
		Height(overlayH).
		Padding(1, 2)

	titleStyle := lipgloss.NewStyle().
		Foreground(ColorAccent).
		Bold(true)

	subtitleStyle := lipgloss.NewStyle().
		Foreground(ColorTextDim)

	content := titleStyle.Render("  Devices") + "\n"
	content += subtitleStyle.Render("  Select a playback device") + "\n\n"

	if len(d.devices) == 0 {
		content += subtitleStyle.Render("  No devices available") + "\n"
	} else {
		for i, dev := range d.devices {
			prefix := "  "
			style := lipgloss.NewStyle().Foreground(ColorTextSec)
			if i == d.cursor {
				prefix = "> "
				style = StyleActiveItem
			}

			icon := deviceIcon(dev.Type)
			name := dev.Name
			if dev.IsActive {
				name += "  *"
			}
			line := fmt.Sprintf("%s%s  %s", prefix, icon, name)
			content += style.Render(line) + "\n"
		}
	}

	content += "\n" + subtitleStyle.Render("  j/k navigate  Enter select  Esc close")

	overlay := border.Render(content)
	return lipgloss.Place(d.width, d.height, lipgloss.Center, lipgloss.Center, overlay,
		lipgloss.WithWhitespaceBackground(lipgloss.Color("#000000")))
}

// deviceIcon returns an icon for the device type.
func deviceIcon(deviceType string) string {
	switch deviceType {
	case "Computer":
		return "[PC]"
	case "Smartphone":
		return "[Phone]"
	case "Speaker":
		return "[Speaker]"
	case "TV":
		return "[TV]"
	case "CastAudio", "CastVideo":
		return "[Cast]"
	default:
		return "[" + deviceType + "]"
	}
}
