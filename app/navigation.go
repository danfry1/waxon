package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/danfry1/waxon/source"
)

const (
	trackCacheTTL        = 5 * time.Minute
	maxTrackCacheEntries = 50
	maxNavStack          = 20
)

type cachedPlaylist struct {
	tracks    []source.Track
	total     int
	fetchedAt time.Time
}

// evictTrackCache removes the oldest cache entries to stay under maxTrackCacheEntries.
func (m *Model) evictTrackCache() {
	for len(m.trackCache) > maxTrackCacheEntries {
		var oldestKey string
		var oldestTime time.Time
		for k, v := range m.trackCache {
			if oldestKey == "" || v.fetchedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.fetchedAt
			}
		}
		delete(m.trackCache, oldestKey)
	}
}

type paginationState struct {
	playlistID  string
	contextURI  string
	imageURL    string
	title       string
	total       int
	loaded      int
	loadingMore bool
}

// navEntry is one back-stack frame: the tracklist snapshot plus the lazy-load
// state that belongs to it. Pagination must travel with the view — otherwise
// going back and scrolling would fetch (and splice in) pages of whatever
// playlist was being paginated *before* the back navigation.
type navEntry struct {
	state      NavState
	pagination *paginationState // nil when the view was fully loaded
}

// pushNav saves the current tracklist state onto the navigation stack.
func (m *Model) pushNav() {
	if len(m.tracklist.tracks) == 0 {
		return
	}
	entry := navEntry{state: m.tracklist.GetState(m.focusPane)}
	if m.pagination != nil {
		pg := *m.pagination
		pg.loadingMore = false // any in-flight page belongs to the old view; ignore it
		entry.pagination = &pg
	}
	// The view being navigated to owns its own pagination (set when it loads).
	m.pagination = nil
	m.navStack = append(m.navStack, entry)
	if len(m.navStack) > maxNavStack {
		m.navStack = m.navStack[len(m.navStack)-maxNavStack:]
	}
}

// applyPendingJump moves the cursor to a track that was requested before its
// view finished loading (see jumpToCurrentTrack). It is a no-op when nothing is
// pending. When the track isn't on the loaded page yet but more pages are still
// arriving, it keeps the jump pending and pulls the next page so a later load
// can complete it. Once the view is fully loaded and the track still isn't
// present, it clears the pending state and reports that the track is absent.
func (m *Model) applyPendingJump() tea.Cmd {
	if m.pendingJumpTrackID == "" {
		return nil
	}
	m.focusPane = PaneTrackList
	if m.tracklist.JumpToTrack(m.pendingJumpTrackID) {
		m.pendingJumpTrackID = ""
		return nil
	}
	// Not on the loaded page yet — keep waiting while more pages stream in.
	if m.pagination != nil && m.pagination.loaded < m.pagination.total {
		if m.pagination.loadingMore {
			return nil
		}
		m.pagination.loadingMore = true
		return m.fetchMoreTracks()
	}
	// Fully loaded and still missing — the track isn't in this context.
	m.pendingJumpTrackID = ""
	name := ""
	if m.track != nil {
		name = m.track.Name
	}
	m.toast.Show("Track not in current view", name, ToastInfo)
	return scheduleAutoDismiss()
}

// popNav restores the previous tracklist state from the navigation stack.
func (m *Model) popNav() bool {
	if len(m.navStack) == 0 {
		return false
	}
	last := len(m.navStack) - 1
	entry := m.navStack[last]
	m.navStack = m.navStack[:last]
	m.tracklist.RestoreState(entry.state)
	m.focusPane = entry.state.focusPane
	m.pagination = entry.pagination
	if m.track != nil {
		m.tracklist.SetNowPlaying(m.track.ID)
	}
	return true
}
