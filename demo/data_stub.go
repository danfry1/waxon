//go:build demo

package demo

import (
	"time"

	"github.com/danielfry/waxon/source"
)

func buildPlaylists() ([]source.Playlist, map[string][]source.Track, []source.Track) {
	pl := []source.Playlist{
		{ID: "pl-1", URI: "spotify:playlist:1", Name: "Demo Playlist", TrackCount: 2},
	}
	tracks := []source.Track{
		{ID: "t1", URI: "spotify:track:t1", Name: "Demo Track 1", Artist: "Demo Artist", Album: "Demo Album", Duration: 3 * time.Minute},
		{ID: "t2", URI: "spotify:track:t2", Name: "Demo Track 2", Artist: "Demo Artist", Album: "Demo Album", Duration: 4 * time.Minute},
	}
	return pl, map[string][]source.Track{"pl-1": tracks}, tracks
}

func buildArtists() map[string]*source.ArtistPage {
	return map[string]*source.ArtistPage{}
}

func buildAlbums() map[string]*source.AlbumPage {
	return map[string]*source.AlbumPage{}
}

func buildDevices() []source.Device {
	return []source.Device{
		{ID: "dev-1", Name: "MacBook Pro", Type: "Computer", IsActive: true},
	}
}
