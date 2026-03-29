package source

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const spotifyScript = `if application "Spotify" is running then
	tell application "Spotify"
		if player state is stopped then
			return "stopped"
		end if
		return (name of current track) & "\n" & (artist of current track) & "\n" & (album of current track) & "\n" & (duration of current track) & "\n" & (player position) & "\n" & (player state as string) & "\n" & (artwork url of current track)
	end tell
else
	return "not_running"
end if`

type LocalSource struct{}

func NewLocalSource() *LocalSource {
	return &LocalSource{}
}

func (s *LocalSource) CurrentTrack() (*Track, error) {
	out, err := runOsascript(spotifyScript)
	if err != nil {
		return nil, fmt.Errorf("osascript: %w", err)
	}
	return parseOutput(out)
}

func (s *LocalSource) Play() error {
	_, err := runOsascript(`tell application "Spotify" to play`)
	return err
}

func (s *LocalSource) Pause() error {
	_, err := runOsascript(`tell application "Spotify" to pause`)
	return err
}

func (s *LocalSource) Next() error {
	_, err := runOsascript(`tell application "Spotify" to next track`)
	return err
}

func (s *LocalSource) Previous() error {
	_, err := runOsascript(`tell application "Spotify" to previous track`)
	return err
}

func (s *LocalSource) Seek(position time.Duration) error {
	script := fmt.Sprintf(`tell application "Spotify" to set player position to %f`, position.Seconds())
	_, err := runOsascript(script)
	return err
}

func runOsascript(script string) (string, error) {
	cmd := exec.Command("osascript", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func parseOutput(raw string) (*Track, error) {
	if raw == "not_running" || raw == "stopped" {
		return nil, nil
	}

	lines := strings.SplitN(raw, "\n", 7)
	if len(lines) < 6 {
		return nil, fmt.Errorf("unexpected output format: got %d lines", len(lines))
	}

	durationMs, err := strconv.ParseFloat(lines[3], 64)
	if err != nil {
		return nil, fmt.Errorf("parse duration: %w", err)
	}

	positionSec, err := strconv.ParseFloat(lines[4], 64)
	if err != nil {
		return nil, fmt.Errorf("parse position: %w", err)
	}

	artworkURL := ""
	if len(lines) >= 7 {
		artworkURL = lines[6]
	}

	return &Track{
		Name:       lines[0],
		Artist:     lines[1],
		Album:      lines[2],
		ArtworkURL: artworkURL,
		Duration:   time.Duration(durationMs) * time.Millisecond,
		Position:   time.Duration(positionSec * float64(time.Second)),
		Playing:    lines[5] == "playing",
	}, nil
}
