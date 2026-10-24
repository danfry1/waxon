package app

import (
	"crypto/sha1" //nolint:gosec // file name, not security
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

// Page cache: the first page of every playlist opened is kept on disk with
// the playlist's version — Spotify's snapshot_id, which the library listing
// carries anyway. Opening a playlist whose version still matches shows it
// from disk without an API call; a stale or unversioned copy (Liked Songs
// has no version) shows at once while the live page is fetched to replace
// it. Pages beyond the first always load live.

const maxDiskPages = 60

type diskPage struct {
	Version string         `json:"version"`
	Tracks  []source.Track `json:"tracks"`
	Total   int            `json:"total"`
}

// pageMissMsg means no usable page was on disk; load pl live.
type pageMissMsg struct{ playlist source.Playlist }

// pageVersion is what a cached page of pl must match to be current; "" when
// pl's contents have no version, and the page is always revalidated. Liked
// Songs has none: its count can't tell a like and an unlike apart.
func pageVersion(pl source.Playlist) string {
	if pl.Snapshot != "" {
		return "s:" + pl.Snapshot
	}
	return ""
}

// pageDir is where pages live: next to the library snapshot. "" disables
// the page cache.
func (m Model) pageDir() string {
	if m.opts.SnapshotPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(m.opts.SnapshotPath), "pages")
}

func pagePath(dir, playlistID string) string {
	sum := sha1.Sum([]byte(playlistID)) //nolint:gosec // file name, not security
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".json")
}

func readPage(dir, playlistID string) (*diskPage, error) {
	data, err := os.ReadFile(pagePath(dir, playlistID))
	if err != nil {
		return nil, err
	}
	var p diskPage
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func writePage(dir, playlistID string, p diskPage) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "page-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, err = f.Write(data)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, pagePath(dir, playlistID))
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}

// prunePages keeps the maxDiskPages most recently used pages (a cache hit
// touches its page).
func prunePages(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type page struct {
		path string
		mod  int64
	}
	pages := make([]page, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		switch path := filepath.Join(dir, e.Name()); filepath.Ext(e.Name()) {
		case ".json":
			pages = append(pages, page{path, info.ModTime().UnixNano()})
		case ".tmp":
			if time.Since(info.ModTime()) > time.Hour {
				_ = os.Remove(path) // left by a write that never finished
			}
		}
	}
	if len(pages) <= maxDiskPages {
		return
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i].mod > pages[j].mod })
	for _, p := range pages[maxDiskPages:] {
		_ = os.Remove(p.path)
	}
}

// loadPlaylistPage shows pl from its disk page, if there is one: current
// (no fetch needed) or stale (the live page follows). Otherwise pl loads
// live.
func (m Model) loadPlaylistPage(pl source.Playlist) tea.Cmd {
	dir := m.pageDir()
	return func() tea.Msg {
		p, err := readPage(dir, pl.ID)
		if err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				slog.Warn("ignoring cached playlist page", "playlist", pl.ID, "error", err)
			}
			return pageMissMsg{pl}
		}
		want := pageVersion(pl)
		now := time.Now()
		_ = os.Chtimes(pagePath(dir, pl.ID), now, now) // recently used: pruned last
		return tracksLoadedMsg{
			tracks:     p.Tracks,
			title:      pl.Name,
			contextURI: pl.URI,
			imageURL:   pl.ImageURL,
			playlistID: pl.ID,
			total:      p.Total,
			playlist:   pl,
			version:    p.Version,
			fromDisk:   true,
			current:    want != "" && p.Version == want,
		}
	}
}

// savePage writes a live first page to disk.
func (m Model) savePage(msg tracksLoadedMsg) tea.Cmd {
	dir := m.pageDir()
	if dir == "" || msg.playlistID == "" {
		return nil
	}
	p := diskPage{Version: msg.version, Tracks: msg.tracks, Total: msg.total}
	id := msg.playlistID
	return func() tea.Msg {
		if err := writePage(dir, id, p); err != nil {
			slog.Warn("caching playlist page", "playlist", id, "error", err)
		}
		return nil
	}
}

// forgetPage drops a playlist's cached pages after waxon changed it, so
// the edit can't be shown undone.
func (m *Model) forgetPage(playlistID string) tea.Cmd {
	delete(m.trackCache, playlistID)
	dir := m.pageDir()
	if dir == "" {
		return nil
	}
	return func() tea.Msg {
		_ = os.Remove(pagePath(dir, playlistID))
		return nil
	}
}
