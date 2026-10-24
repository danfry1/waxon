package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Startup snapshot: waxon saves the library list and which playlist was open
// (with the cursor), and the next launch shows them before the first API call
// returns — the playlist from its cached page (pagecache.go). The live load
// then corrects whatever changed, so the snapshot only has to be recent.

const snapshotVersion = 2

type librarySnapshot struct {
	Version   int               `json:"version"`
	Playlists []source.Playlist `json:"playlists"`
	Last      string            `json:"last,omitempty"`   // playlist open at the last save
	Cursor    int               `json:"cursor,omitempty"` // its cursor, saved on quit
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

// RemoveSnapshot deletes the snapshot at path and the cached playlist pages
// beside it, so a different account's library is never shown. Missing files
// are not an error.
func RemoveSnapshot(path string) error {
	if path == "" {
		return nil
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.RemoveAll(filepath.Join(filepath.Dir(path), "pages"))
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
	pages := m.pageDir()
	return func() tea.Msg {
		go prunePages(pages)
		snap, err := readSnapshot(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("ignoring library snapshot", "path", path, "error", err)
		}
		return snapshotLoadedMsg{snap: snap}
	}
}

// saveSnapshot writes the live library and the open playlist off the main
// loop. Failures only cost the next launch its head start.
func (m Model) saveSnapshot() tea.Cmd {
	path := m.opts.SnapshotPath
	if path == "" || !m.libraryLive || len(m.playlists) == 0 {
		return nil
	}
	// A copy: the library slice is edited in place (track counts) while the
	// write runs.
	snap := librarySnapshot{Playlists: slices.Clone(m.playlists), Last: m.lastPlaylistID}
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

// saveSnapshotOnQuit writes the snapshot with the cursor, synchronously: the
// program is about to exit, so a tea.Cmd wouldn't run.
func (m Model) saveSnapshotOnQuit() {
	path := m.opts.SnapshotPath
	if path == "" || !m.libraryLive || len(m.playlists) == 0 {
		return
	}
	snap := librarySnapshot{Playlists: m.playlists, Last: m.lastPlaylistID}
	if pl := m.playlistByID(m.lastPlaylistID); pl != nil && m.tracklist.contextURI == pl.URI && !m.tracklist.Filtering() {
		snap.Cursor = m.tracklist.Cursor()
	}
	snapshotMu.Lock()
	defer snapshotMu.Unlock()
	snapshotSeq++
	snapshotWritten = snapshotSeq
	if err := writeSnapshot(path, snap); err != nil {
		slog.Warn("saving library snapshot", "path", path, "error", err)
	}
}

// applySnapshot shows a startup snapshot — the library, and the playlist that
// was open, from its cached page — unless the live library already arrived
// (the two loads race).
func (m Model) applySnapshot(snap *librarySnapshot) (Model, tea.Cmd) {
	if snap == nil || m.libraryLive {
		return m, nil
	}
	m.playlists = snap.Playlists
	if m.sidebar.width > 0 {
		m.sidebar.SetPlaylists(snap.Playlists)
	}
	cmds := []tea.Cmd{m.fetchSidebarIcons(snap.Playlists)}
	if len(m.tracklist.tracks) == 0 && m.tracklist.contextURI == "" {
		pl := m.playlistByID(snap.Last)
		if pl == nil {
			pl = &m.playlists[0]
		}
		m.lastPlaylistID = pl.ID
		m.resumePlaylistID, m.resumeCursor = pl.ID, snap.Cursor
		m.snapshotShown = true
		m.tracklist.SetLoadingFor(pl.Name, pl.URI)
		cmds = append(cmds, m.fetchPlaylistTracks(*pl))
	}
	return m, tea.Batch(cmds...)
}

// showingLoaded reports whether the track list shows uri, loaded (not on the
// way to another view).
func (m Model) showingLoaded(uri string) bool {
	return !m.tracklist.loading && m.tracklist.contextURI == uri
}

// samePage reports whether live's tracks are the first tracks of shown.
func samePage(shown, live []source.Track) bool {
	if len(live) > len(shown) {
		return false
	}
	for i := range live {
		if shown[i].ID != live[i].ID {
			return false
		}
	}
	return true
}

// shownTotal is the size of the playlist on screen: its total when more
// pages are still to load, else what's shown.
func (m Model) shownTotal() int {
	if m.pagination != nil {
		return m.pagination.total
	}
	return len(m.tracklist.tracks)
}

// playlistByID finds a library playlist (nil if it isn't in the library).
func (m Model) playlistByID(id string) *source.Playlist {
	for i := range m.playlists {
		if m.playlists[i].ID == id {
			return &m.playlists[i]
		}
	}
	return nil
}

// playlistByURI finds a library playlist by its URI.
func (m Model) playlistByURI(uri string) *source.Playlist {
	for i := range m.playlists {
		if uri != "" && m.playlists[i].URI == uri {
			return &m.playlists[i]
		}
	}
	return nil
}
