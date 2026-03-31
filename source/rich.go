package source

import "context"

// PlaybackSource provides playback control beyond basic TrackSource operations.
type PlaybackSource interface {
	SetVolume(ctx context.Context, percent int) error
	SetShuffle(ctx context.Context, state bool) error
	SetRepeat(ctx context.Context, mode RepeatMode) error
	Devices(ctx context.Context) ([]Device, error)
	TransferPlayback(ctx context.Context, deviceID string) error
	PlayTrack(ctx context.Context, contextURI string, trackURI string) error
	PlayTrackDirect(ctx context.Context, trackURI string) error
	AddToQueue(ctx context.Context, trackID string) error
}

// LibrarySource provides access to the user's music library.
type LibrarySource interface {
	Queue(ctx context.Context) ([]Track, error)
	Playlists(ctx context.Context) ([]Playlist, error)
	PlaylistTracksPage(ctx context.Context, id string, offset, limit int) ([]Track, int, error)
	LikedTracksPage(ctx context.Context, offset, limit int) ([]Track, int, error)
	RecentlyPlayed(ctx context.Context) ([]Track, error)
}

// SearchSource provides search and browse capabilities.
type SearchSource interface {
	Search(ctx context.Context, query string) (*SearchResults, error)
	AudioFeatures(ctx context.Context, trackID string) (*AudioFeatures, error)
	GetArtist(ctx context.Context, artistID string) (*ArtistPage, error)
	GetAlbum(ctx context.Context, albumID string) (*AlbumPage, error)
}

// RichSource combines all source capabilities. Implementations must satisfy
// TrackSource, PlaybackSource, LibrarySource, and SearchSource.
type RichSource interface {
	TrackSource
	PlaybackSource
	LibrarySource
	SearchSource
}

// ArtistAlbum represents a simplified album in an artist's discography.
type ArtistAlbum struct {
	ID       string
	Name     string
	Year     string // extracted from release_date
	Type     string // "Album" or "Single"
	ImageURL string
}

// ArtistPage holds data for the artist detail view.
type ArtistPage struct {
	Name     string
	ImageURL string
	Genres   []string
	Tracks   []Track
	Albums   []ArtistAlbum
}

// AlbumPage holds data for the album detail view.
type AlbumPage struct {
	ID       string
	Name     string
	Artist   string
	Year     string // release year (first 4 chars of release_date)
	ImageURL string
	Tracks   []Track
}

type Playlist struct {
	ID         string
	URI        string
	Name       string
	ImageURL   string
	TrackCount int
}

type Device struct {
	ID       string
	Name     string
	Type     string
	IsActive bool
}

type SearchArtist struct {
	ID       string
	Name     string
	ImageURL string
}

type SearchAlbum struct {
	ID       string
	Name     string
	Artist   string
	ImageURL string
}

type SearchResults struct {
	Tracks  []Track
	Artists []SearchArtist
	Albums  []SearchAlbum
}

type AudioFeatures struct {
	Energy       float64
	Valence      float64
	Danceability float64
	Tempo        float64
	Acousticness float64
}

type RepeatMode string

const (
	RepeatOff     RepeatMode = "off"
	RepeatContext RepeatMode = "context"
	RepeatTrack   RepeatMode = "track"
)
