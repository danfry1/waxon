package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Startup snapshot: after each live load waxon saves the library list and the
// first page of the first playlist (Liked Songs), and the next launch draws
// them from disk before the first API call returns. The live load then
// replaces them, so the snapshot only has to be recent, never exact.

const snapshotVersion = 1

type librarySnapshot struct {
	Version   int               `json:"version"`
	Playlists []source.Playlist `json:"playlists"`
	Tracks    []source.Track    `json:"tracks"` // first page of Playlists[0]
	Total     int               `json:"total"`  // Playlists[0]'s size
}

// snapshotLoadedMsg carries the snapshot read at startup (nil if none).
type snapshotLoadedMsg struct{ snap *librarySnapshot }

// DefaultSnapshotPath returns $XDG_CACHE_HOME/waxon/library.json (or the OS
// user cache dir equivalent). Returns "" if no cache dir can be determined.
func DefaultSnapshotPath() string {
	dir := DefaultImageCacheDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(dir), "library.json")
}

// RemoveSnapshot deletes the snapshot at path, so a different account's
// library is never shown. A missing file is not an error.
func RemoveSnapshot(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func readSnapshot(path string) (*librarySnapshot, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var snap librarySnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	if snap.Version != snapshotVersion || len(snap.Playlists) == 0 {
		return nil, nil
	}
	return &snap, nil
}

// Saves run as concurrent tea.Cmds: snapshotMu serialises the writes, and a
// save that runs after a newer one (by snapshotSeq) is dropped.
var (
	snapshotMu      sync.Mutex
	snapshotSeq     uint64 // last save handed out (main loop only)
	snapshotWritten uint64 // newest save on disk (under snapshotMu)
)

func writeSnapshot(path string, snap librarySnapshot) error {
	snap.Version = snapshotVersion
	data, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "library-*.tmp") // created 0600
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

func (m Model) loadSnapshot() tea.Cmd {
	path := m.opts.SnapshotPath
	if path == "" {
		return nil
	}
	return func() tea.Msg {
		snap, err := readSnapshot(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("ignoring library snapshot", "path", path, "error", err)
		}
		return snapshotLoadedMsg{snap: snap}
	}
}

// saveSnapshot writes the live library and first-playlist page off the main
// loop. Failures only cost the next launch its head start.
func (m Model) saveSnapshot() tea.Cmd {
	path := m.opts.SnapshotPath
	if path == "" || !m.libraryLive || len(m.playlists) == 0 {
		return nil
	}
	snap := librarySnapshot{Playlists: m.playlists, Tracks: m.snapTracks, Total: m.snapTotal}
	snapshotSeq++
	seq := snapshotSeq
	return func() tea.Msg {
		snapshotMu.Lock()
		defer snapshotMu.Unlock()
		if seq < snapshotWritten {
			return nil
		}
		snapshotWritten = seq
		if err := writeSnapshot(path, snap); err != nil {
			slog.Warn("saving library snapshot", "path", path, "error", err)
		}
		return nil
	}
}

// applySnapshot shows a startup snapshot unless the live library already
// arrived (the two loads race) or something else is already on screen.
func (m Model) applySnapshot(snap *librarySnapshot) (Model, tea.Cmd) {
	if snap == nil || m.libraryLive {
		return m, nil
	}
	m.playlists = snap.Playlists
	if m.sidebar.width > 0 {
		m.sidebar.SetPlaylists(snap.Playlists)
	}
	m.snapTracks, m.snapTotal = snap.Tracks, snap.Total
	m.showSnapshotPage()
	return m, m.fetchSidebarIcons(snap.Playlists)
}

// showSnapshotPage opens the track list on the snapshot's first-playlist page
// if nothing else is on screen yet. Scrolling past it loads the rest live.
func (m *Model) showSnapshotPage() {
	if m.libraryLive || len(m.snapTracks) == 0 || len(m.playlists) == 0 ||
		len(m.tracklist.tracks) > 0 || m.tracklist.contextURI != "" {
		return
	}
	first, tracks, total := m.playlists[0], m.snapTracks, m.snapTotal
	m.tracklist.SetTracks(tracks, first.Name, first.URI)
	if total > len(tracks) {
		m.pagination = &paginationState{
			playlistID: first.ID,
			contextURI: first.URI,
			imageURL:   first.ImageURL,
			title:      first.Name,
			total:      total,
			loaded:     len(tracks),
		}
		m.tracklist.SetHeaderInfo(partialListInfo(tracks, len(tracks), total))
	} else {
		m.tracklist.SetHeaderInfo(FormatTrackListInfo(tracks))
	}
	m.snapshotShown = true
}

// supersededReload reports whether msg is the live reload of the snapshot's
// page arriving after the user opened something else, which it must not
// replace. Its data is still cached.
func (m *Model) supersededReload(msg tracksLoadedMsg) bool {
	if !m.snapshotReload || len(m.playlists) == 0 || msg.playlistID != m.playlists[0].ID {
		return false
	}
	m.snapshotReload = false
	return m.tracklist.loading || m.tracklist.contextURI != msg.contextURI
}
