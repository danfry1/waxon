package app

import (
	"time"

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

// pushNav saves the current tracklist state onto the navigation stack.
func (m *Model) pushNav() {
	if len(m.tracklist.tracks) == 0 {
		return
	}
	state := m.tracklist.GetState(m.focusPane)
	m.navStack = append(m.navStack, state)
	if len(m.navStack) > maxNavStack {
		m.navStack = m.navStack[len(m.navStack)-maxNavStack:]
	}
}

// popNav restores the previous tracklist state from the navigation stack.
func (m *Model) popNav() bool {
	if len(m.navStack) == 0 {
		return false
	}
	last := len(m.navStack) - 1
	state := m.navStack[last]
	m.navStack = m.navStack[:last]
	m.tracklist.RestoreState(state)
	m.focusPane = state.focusPane
	if m.track != nil {
		m.tracklist.SetNowPlaying(m.track.ID)
	}
	return true
}
