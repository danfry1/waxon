//go:build demo

package demo

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/danielfry/waxon/source"
)

// DemoSource implements source.RichSource with curated fake data and
// simulated playback. No network calls are made.
type DemoSource struct {
	mu sync.Mutex

	// Playback state
	playing    bool
	trackIdx   int
	position   time.Duration
	startTime  time.Time
	volume     int
	shuffleOn  bool
	repeatMode source.RepeatMode

	// Content
	playlists  []source.Playlist
	tracksByPL map[string][]source.Track
	queue      []source.Track
	devices    []source.Device
	artists    map[string]*source.ArtistPage
	albums     map[string]*source.AlbumPage
	allTracks  []source.Track

	// Current context
	currentPLID string
}

// NewDemoSource creates a fully populated demo source ready for use.
func NewDemoSource() *DemoSource {
	pl, tracksByPL, allTracks := buildPlaylists()
	artists := buildArtists()
	albums := buildAlbums()
	devices := buildDevices()

	firstPL := pl[0].ID
	firstTracks := tracksByPL[firstPL]

	ds := &DemoSource{
		playing:    true,
		trackIdx:   0,
		position:   0,
		startTime:  time.Now(),
		volume:     65,
		repeatMode: source.RepeatOff,

		playlists:   pl,
		tracksByPL:  tracksByPL,
		allTracks:   allTracks,
		queue:       firstTracks[1:min(6, len(firstTracks))],
		devices:     devices,
		artists:     artists,
		albums:      albums,
		currentPLID: firstPL,
	}
	return ds
}

func (d *DemoSource) currentTrack() *source.Track {
	tracks := d.tracksByPL[d.currentPLID]
	if len(tracks) == 0 {
		return nil
	}
	idx := d.trackIdx % len(tracks)
	t := tracks[idx]
	return &t
}

func (d *DemoSource) computePosition() time.Duration {
	if !d.playing {
		return d.position
	}
	elapsed := time.Since(d.startTime)
	pos := d.position + elapsed

	track := d.currentTrack()
	if track != nil && pos >= track.Duration {
		d.advanceTrack()
		return 0
	}
	return pos
}

func (d *DemoSource) advanceTrack() {
	tracks := d.tracksByPL[d.currentPLID]
	if len(tracks) == 0 {
		return
	}
	d.trackIdx = (d.trackIdx + 1) % len(tracks)
	d.position = 0
	d.startTime = time.Now()
	remaining := len(tracks) - d.trackIdx - 1
	if remaining > 5 {
		remaining = 5
	}
	if remaining > 0 {
		d.queue = make([]source.Track, remaining)
		copy(d.queue, tracks[d.trackIdx+1:d.trackIdx+1+remaining])
	} else {
		d.queue = nil
	}
}

// --- source.TrackSource ---

func (d *DemoSource) CurrentPlayback(_ context.Context) (*source.PlaybackState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	track := d.currentTrack()
	if track == nil {
		return nil, nil
	}

	pos := d.computePosition()
	t := *track
	t.Position = pos
	t.Playing = d.playing
	t.DeviceName = d.devices[0].Name

	return &source.PlaybackState{
		Track:      &t,
		Volume:     d.volume,
		ShuffleOn:  d.shuffleOn,
		RepeatMode: d.repeatMode,
	}, nil
}

func (d *DemoSource) Play(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.playing {
		d.playing = true
		d.startTime = time.Now()
	}
	return nil
}

func (d *DemoSource) Pause(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.playing {
		d.position = d.computePosition()
		d.playing = false
	}
	return nil
}

func (d *DemoSource) Next(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.advanceTrack()
	return nil
}

func (d *DemoSource) Previous(_ context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	tracks := d.tracksByPL[d.currentPLID]
	if len(tracks) == 0 {
		return nil
	}
	d.trackIdx--
	if d.trackIdx < 0 {
		d.trackIdx = len(tracks) - 1
	}
	d.position = 0
	d.startTime = time.Now()
	return nil
}

func (d *DemoSource) Seek(_ context.Context, pos time.Duration) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.position = pos
	d.startTime = time.Now()
	return nil
}

// --- source.PlaybackSource ---

func (d *DemoSource) SetVolume(_ context.Context, percent int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.volume = percent
	return nil
}

func (d *DemoSource) SetShuffle(_ context.Context, state bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.shuffleOn = state
	return nil
}

func (d *DemoSource) SetRepeat(_ context.Context, mode source.RepeatMode) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.repeatMode = mode
	return nil
}

func (d *DemoSource) Devices(_ context.Context) ([]source.Device, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.devices, nil
}

func (d *DemoSource) TransferPlayback(_ context.Context, deviceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.devices {
		d.devices[i].IsActive = d.devices[i].ID == deviceID
	}
	return nil
}

func (d *DemoSource) PlayTrack(_ context.Context, contextURI, trackURI string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, pl := range d.playlists {
		if pl.URI == contextURI {
			tracks := d.tracksByPL[pl.ID]
			for i, t := range tracks {
				if t.URI == trackURI {
					d.currentPLID = pl.ID
					d.trackIdx = i
					d.position = 0
					d.startTime = time.Now()
					d.playing = true
					return nil
				}
			}
			d.currentPLID = pl.ID
			d.trackIdx = 0
			d.position = 0
			d.startTime = time.Now()
			d.playing = true
			return nil
		}
	}
	return nil
}

func (d *DemoSource) PlayTrackDirect(_ context.Context, trackURI string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for plID, tracks := range d.tracksByPL {
		for i, t := range tracks {
			if t.URI == trackURI {
				d.currentPLID = plID
				d.trackIdx = i
				d.position = 0
				d.startTime = time.Now()
				d.playing = true
				return nil
			}
		}
	}
	return nil
}

func (d *DemoSource) AddToQueue(_ context.Context, trackID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, t := range d.allTracks {
		if t.ID == trackID {
			d.queue = append(d.queue, t)
			return nil
		}
	}
	return nil
}

// --- source.LibrarySource ---

func (d *DemoSource) Queue(_ context.Context) ([]source.Track, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	track := d.currentTrack()
	if track == nil {
		return d.queue, nil
	}
	return append([]source.Track{*track}, d.queue...), nil
}

func (d *DemoSource) Playlists(_ context.Context) ([]source.Playlist, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.playlists, nil
}

func (d *DemoSource) PlaylistTracksPage(_ context.Context, id string, offset, limit int) ([]source.Track, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	tracks := d.tracksByPL[id]
	total := len(tracks)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return tracks[offset:end], total, nil
}

func (d *DemoSource) LikedTracksPage(_ context.Context, offset, limit int) ([]source.Track, int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.playlists) == 0 {
		return nil, 0, nil
	}
	tracks := d.tracksByPL[d.playlists[0].ID]
	total := len(tracks)
	if offset >= total {
		return nil, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return tracks[offset:end], total, nil
}

func (d *DemoSource) RecentlyPlayed(_ context.Context) ([]source.Track, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.allTracks)
	if n > 10 {
		return d.allTracks[n-10:], nil
	}
	return d.allTracks, nil
}

// --- source.SearchSource ---

func (d *DemoSource) Search(_ context.Context, query string) (*source.SearchResults, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	q := strings.ToLower(query)
	var tracks []source.Track
	for _, t := range d.allTracks {
		if strings.Contains(strings.ToLower(t.Name), q) ||
			strings.Contains(strings.ToLower(t.Artist), q) ||
			strings.Contains(strings.ToLower(t.Album), q) {
			tracks = append(tracks, t)
			if len(tracks) >= 20 {
				break
			}
		}
	}

	var artists []source.SearchArtist
	for id, a := range d.artists {
		if strings.Contains(strings.ToLower(a.Name), q) {
			artists = append(artists, source.SearchArtist{
				ID:       id,
				Name:     a.Name,
				ImageURL: a.ImageURL,
			})
		}
	}

	var albums []source.SearchAlbum
	for id, a := range d.albums {
		if strings.Contains(strings.ToLower(a.Name), q) ||
			strings.Contains(strings.ToLower(a.Artist), q) {
			albums = append(albums, source.SearchAlbum{
				ID:       id,
				Name:     a.Name,
				Artist:   a.Artist,
				ImageURL: a.ImageURL,
			})
		}
	}

	return &source.SearchResults{
		Tracks:  tracks,
		Artists: artists,
		Albums:  albums,
	}, nil
}

func (d *DemoSource) AudioFeatures(_ context.Context, _ string) (*source.AudioFeatures, error) {
	return &source.AudioFeatures{
		Energy:       0.72,
		Valence:      0.64,
		Danceability: 0.55,
		Tempo:        120.0,
		Acousticness: 0.18,
	}, nil
}

func (d *DemoSource) GetArtist(_ context.Context, artistID string) (*source.ArtistPage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if page, ok := d.artists[artistID]; ok {
		return page, nil
	}
	return &source.ArtistPage{Name: "Unknown Artist"}, nil
}

func (d *DemoSource) GetAlbum(_ context.Context, albumID string) (*source.AlbumPage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if page, ok := d.albums[albumID]; ok {
		return page, nil
	}
	return &source.AlbumPage{Name: "Unknown Album"}, nil
}

// Compile-time interface check.
var _ source.RichSource = (*DemoSource)(nil)
