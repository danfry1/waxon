package source

type RichSource interface {
	TrackSource
	Queue() ([]Track, error)
	Playlists() ([]Playlist, error)
	PlaylistTracks(id string) ([]Track, error)
	Search(query string) (*SearchResults, error)
	SetVolume(percent int) error
	Devices() ([]Device, error)
	TransferPlayback(deviceID string) error
	SetShuffle(state bool) error
	SetRepeat(mode RepeatMode) error
	AudioFeatures(trackID string) (*AudioFeatures, error)
}

type Playlist struct {
	ID         string
	Name       string
	TrackCount int
}

type Device struct {
	ID       string
	Name     string
	Type     string
	IsActive bool
}

type SearchResults struct {
	Tracks  []Track
	Artists []string
	Albums  []string
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
