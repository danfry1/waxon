//go:build demo

package demo

import (
	"time"

	"github.com/danielfry/waxon/source"
)

// Artwork URLs use a demo:// scheme intercepted by ArtworkProvider.
// Album artwork URLs.
const (
	artDarkSide = "demo://art/dark-side-of-the-moon.jpg"
	artRumours  = "demo://art/rumours.jpg"
	artOKComp   = "demo://art/ok-computer.jpg"
	artKindBlue   = "demo://art/kind-of-blue.jpg"
	artCurrents   = "demo://art/currents.jpg"
	artPurpleRain = "demo://art/purple-rain.jpg"
	artRAM        = "demo://art/random-access-memories.jpg"
	artNevermind  = "demo://art/nevermind.jpg"
)

// Playlist cover artwork URLs.
const (
	artPLLiked  = "demo://art/liked-songs.jpg"
	artPLRock   = "demo://art/classic-rock.jpg"
	artPLCoding = "demo://art/late-night-coding.jpg"
	artPLJazz   = "demo://art/jazz-essentials.jpg"
)

func buildPlaylists() ([]source.Playlist, map[string][]source.Track, []source.Track) {
	playlists := []source.Playlist{
		{ID: "pl-liked", URI: "spotify:playlist:liked", Name: "Liked Songs", ImageURL: artPLLiked, TrackCount: 21},
		{ID: "pl-rock", URI: "spotify:playlist:rock", Name: "Classic Rock", ImageURL: artPLRock, TrackCount: 14},
		{ID: "pl-coding", URI: "spotify:playlist:coding", Name: "Late Night Coding", ImageURL: artPLCoding, TrackCount: 13},
		{ID: "pl-jazz", URI: "spotify:playlist:jazz", Name: "Jazz Essentials", ImageURL: artPLJazz, TrackCount: 8},
	}

	tracksByPL := map[string][]source.Track{
		"pl-liked": {
			track("t56", "Let It Happen", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 7*time.Minute+46*time.Second),
			track("t61", "Smells Like Teen Spirit", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 5*time.Minute+1*time.Second),
			track("t60", "Get Lucky", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 6*time.Minute+9*time.Second),
			track("t57", "The Less I Know the Better", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 3*time.Minute+36*time.Second),
			track("t59", "Purple Rain", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 8*time.Minute+41*time.Second),
			track("t01", "Comfortably Numb", "Pink Floyd", "The Wall", artDarkSide, "artist-pf", "album-wall", 6*time.Minute+22*time.Second),
			track("t02", "Dreams", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+14*time.Second),
			track("t03", "Paranoid Android", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 6*time.Minute+23*time.Second),
			track("t04", "So What", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+22*time.Second),
			track("t58", "New Person, Same Old Mistakes", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 6*time.Minute+3*time.Second),
			track("t05", "Wish You Were Here", "Pink Floyd", "Wish You Were Here", artDarkSide, "artist-pf", "album-wywh", 5*time.Minute+34*time.Second),
			track("t06", "The Chain", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+28*time.Second),
			track("t07", "Karma Police", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+21*time.Second),
			track("t08", "Blue in Green", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 5*time.Minute+27*time.Second),
			track("t09", "Time", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 7*time.Minute+6*time.Second),
			track("t10", "Go Your Own Way", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+38*time.Second),
			track("t11", "Lucky", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+19*time.Second),
			track("t13", "Money", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 6*time.Minute+22*time.Second),
			track("t14", "Rhiannon", "Fleetwood Mac", "Fleetwood Mac", artRumours, "artist-fm", "album-fmst", 4*time.Minute+12*time.Second),
			track("t12", "Freddie Freeloader", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+46*time.Second),
			track("t15", "No Surprises", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 3*time.Minute+48*time.Second),
		},
		"pl-rock": {
			track("t16", "Bohemian Rhapsody", "Queen", "A Night at the Opera", artDarkSide, "artist-qu", "album-anato", 5*time.Minute+55*time.Second),
			track("t17", "Stairway to Heaven", "Led Zeppelin", "Led Zeppelin IV", artDarkSide, "artist-lz", "album-lz4", 8*time.Minute+2*time.Second),
			track("t18", "Hotel California", "Eagles", "Hotel California", artRumours, "artist-eg", "album-hc", 6*time.Minute+30*time.Second),
			track("t01", "Comfortably Numb", "Pink Floyd", "The Wall", artDarkSide, "artist-pf", "album-wall", 6*time.Minute+22*time.Second),
			track("t19", "Sultans of Swing", "Dire Straits", "Dire Straits", artRumours, "artist-ds", "album-ds1", 5*time.Minute+47*time.Second),
			track("t20", "Whole Lotta Love", "Led Zeppelin", "Led Zeppelin II", artDarkSide, "artist-lz", "album-lz2", 5*time.Minute+34*time.Second),
			track("t02", "Dreams", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+14*time.Second),
			track("t21", "Layla", "Derek and the Dominos", "Layla and Other Assorted Love Songs", artRumours, "artist-ec", "album-layla", 7*time.Minute+10*time.Second),
			track("t22", "Free Bird", "Lynyrd Skynyrd", "Pronounced Leh-Nerd Skin-Nerd", artDarkSide, "artist-ls", "album-pron", 9*time.Minute+8*time.Second),
			track("t09", "Time", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 7*time.Minute+6*time.Second),
			track("t59", "Purple Rain", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 8*time.Minute+41*time.Second),
			track("t62", "When Doves Cry", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 5*time.Minute+54*time.Second),
			track("t61", "Smells Like Teen Spirit", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 5*time.Minute+1*time.Second),
			track("t63", "Come As You Are", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+39*time.Second),
		},
		"pl-coding": {
			track("t03", "Paranoid Android", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 6*time.Minute+23*time.Second),
			track("t23", "Everything In Its Right Place", "Radiohead", "Kid A", artOKComp, "artist-rh", "album-kida", 4*time.Minute+11*time.Second),
			track("t24", "Teardrop", "Massive Attack", "Mezzanine", artOKComp, "artist-ma", "album-mezz", 5*time.Minute+30*time.Second),
			track("t25", "Breathe (In the Air)", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 2*time.Minute+43*time.Second),
			track("t07", "Karma Police", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+21*time.Second),
			track("t26", "Unfinished Sympathy", "Massive Attack", "Blue Lines", artOKComp, "artist-ma", "album-bl", 5*time.Minute+8*time.Second),
			track("t05", "Wish You Were Here", "Pink Floyd", "Wish You Were Here", artDarkSide, "artist-pf", "album-wywh", 5*time.Minute+34*time.Second),
			track("t27", "Idioteque", "Radiohead", "Kid A", artOKComp, "artist-rh", "album-kida", 5*time.Minute+9*time.Second),
			track("t08", "Blue in Green", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 5*time.Minute+27*time.Second),
			track("t11", "Lucky", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+19*time.Second),
			track("t60", "Get Lucky", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 6*time.Minute+9*time.Second),
			track("t64", "Instant Crush", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 5*time.Minute+37*time.Second),
			track("t56", "Let It Happen", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 7*time.Minute+46*time.Second),
		},
		"pl-jazz": {
			track("t04", "So What", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+22*time.Second),
			track("t08", "Blue in Green", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 5*time.Minute+27*time.Second),
			track("t12", "Freddie Freeloader", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+46*time.Second),
			track("t28", "All Blues", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 11*time.Minute+33*time.Second),
			track("t29", "Flamenco Sketches", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+26*time.Second),
			track("t30", "Take Five", "Dave Brubeck", "Time Out", artKindBlue, "artist-db", "album-to", 5*time.Minute+24*time.Second),
			track("t31", "A Love Supreme Pt. 1", "John Coltrane", "A Love Supreme", artKindBlue, "artist-jc", "album-als", 7*time.Minute+43*time.Second),
			track("t32", "Round Midnight", "Thelonious Monk", "Genius of Modern Music", artKindBlue, "artist-tm", "album-gmm", 5*time.Minute+52*time.Second),
		},
	}

	seen := make(map[string]bool)
	var allTracks []source.Track
	for _, pl := range playlists {
		for _, t := range tracksByPL[pl.ID] {
			if !seen[t.ID] {
				seen[t.ID] = true
				allTracks = append(allTracks, t)
			}
		}
	}

	return playlists, tracksByPL, allTracks
}

func track(id, name, artist, album, artURL, artistID, albumID string, dur time.Duration) source.Track {
	return source.Track{
		ID:         id,
		URI:        "spotify:track:" + id,
		Name:       name,
		Artist:     artist,
		Album:      album,
		ArtworkURL: artURL,
		Duration:   dur,
		ArtistID:   artistID,
		AlbumID:    albumID,
	}
}

func buildArtists() map[string]*source.ArtistPage {
	return map[string]*source.ArtistPage{
		"artist-pf": {
			Name:     "Pink Floyd",
			ImageURL: artDarkSide,
			Genres:   []string{"Progressive Rock", "Psychedelic Rock", "Art Rock"},
			Tracks: []source.Track{
				track("t01", "Comfortably Numb", "Pink Floyd", "The Wall", artDarkSide, "artist-pf", "album-wall", 6*time.Minute+22*time.Second),
				track("t09", "Time", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 7*time.Minute+6*time.Second),
				track("t13", "Money", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 6*time.Minute+22*time.Second),
				track("t05", "Wish You Were Here", "Pink Floyd", "Wish You Were Here", artDarkSide, "artist-pf", "album-wywh", 5*time.Minute+34*time.Second),
				track("t25", "Breathe (In the Air)", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 2*time.Minute+43*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-dsotm", Name: "The Dark Side of the Moon", Year: "1973", Type: "Album", ImageURL: artDarkSide},
				{ID: "album-wywh", Name: "Wish You Were Here", Year: "1975", Type: "Album", ImageURL: artDarkSide},
				{ID: "album-wall", Name: "The Wall", Year: "1979", Type: "Album", ImageURL: artDarkSide},
			},
		},
		"artist-fm": {
			Name:     "Fleetwood Mac",
			ImageURL: artRumours,
			Genres:   []string{"Rock", "Pop Rock", "Soft Rock"},
			Tracks: []source.Track{
				track("t02", "Dreams", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+14*time.Second),
				track("t06", "The Chain", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+28*time.Second),
				track("t10", "Go Your Own Way", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+38*time.Second),
				track("t14", "Rhiannon", "Fleetwood Mac", "Fleetwood Mac", artRumours, "artist-fm", "album-fmst", 4*time.Minute+12*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-fmst", Name: "Fleetwood Mac", Year: "1975", Type: "Album", ImageURL: artRumours},
				{ID: "album-rumours", Name: "Rumours", Year: "1977", Type: "Album", ImageURL: artRumours},
			},
		},
		"artist-rh": {
			Name:     "Radiohead",
			ImageURL: artOKComp,
			Genres:   []string{"Alternative Rock", "Art Rock", "Electronic"},
			Tracks: []source.Track{
				track("t03", "Paranoid Android", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 6*time.Minute+23*time.Second),
				track("t07", "Karma Police", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+21*time.Second),
				track("t15", "No Surprises", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 3*time.Minute+48*time.Second),
				track("t23", "Everything In Its Right Place", "Radiohead", "Kid A", artOKComp, "artist-rh", "album-kida", 4*time.Minute+11*time.Second),
				track("t27", "Idioteque", "Radiohead", "Kid A", artOKComp, "artist-rh", "album-kida", 5*time.Minute+9*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-okc", Name: "OK Computer", Year: "1997", Type: "Album", ImageURL: artOKComp},
				{ID: "album-kida", Name: "Kid A", Year: "2000", Type: "Album", ImageURL: artOKComp},
			},
		},
		"artist-md": {
			Name:     "Miles Davis",
			ImageURL: artKindBlue,
			Genres:   []string{"Jazz", "Modal Jazz", "Cool Jazz"},
			Tracks: []source.Track{
				track("t04", "So What", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+22*time.Second),
				track("t08", "Blue in Green", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 5*time.Minute+27*time.Second),
				track("t12", "Freddie Freeloader", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+46*time.Second),
				track("t28", "All Blues", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 11*time.Minute+33*time.Second),
				track("t29", "Flamenco Sketches", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+26*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-kob", Name: "Kind of Blue", Year: "1959", Type: "Album", ImageURL: artKindBlue},
			},
		},
		"artist-ti": {
			Name:     "Tame Impala",
			ImageURL: artCurrents,
			Genres:   []string{"Psychedelic Pop", "Neo-Psychedelia", "Indie Rock"},
			Tracks: []source.Track{
				track("t56", "Let It Happen", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 7*time.Minute+46*time.Second),
				track("t57", "The Less I Know the Better", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 3*time.Minute+36*time.Second),
				track("t58", "New Person, Same Old Mistakes", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 6*time.Minute+3*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-currents", Name: "Currents", Year: "2015", Type: "Album", ImageURL: artCurrents},
			},
		},
		"artist-pr": {
			Name:     "Prince",
			ImageURL: artPurpleRain,
			Genres:   []string{"Pop", "Funk", "Rock"},
			Tracks: []source.Track{
				track("t59", "Purple Rain", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 8*time.Minute+41*time.Second),
				track("t62", "When Doves Cry", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 5*time.Minute+54*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-pr", Name: "Purple Rain", Year: "1984", Type: "Album", ImageURL: artPurpleRain},
			},
		},
		"artist-dp": {
			Name:     "Daft Punk",
			ImageURL: artRAM,
			Genres:   []string{"Electronic", "French House", "Disco"},
			Tracks: []source.Track{
				track("t60", "Get Lucky", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 6*time.Minute+9*time.Second),
				track("t64", "Instant Crush", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 5*time.Minute+37*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-ram", Name: "Random Access Memories", Year: "2013", Type: "Album", ImageURL: artRAM},
			},
		},
		"artist-nv": {
			Name:     "Nirvana",
			ImageURL: artNevermind,
			Genres:   []string{"Grunge", "Alternative Rock", "Punk Rock"},
			Tracks: []source.Track{
				track("t61", "Smells Like Teen Spirit", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 5*time.Minute+1*time.Second),
				track("t63", "Come As You Are", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+39*time.Second),
			},
			Albums: []source.ArtistAlbum{
				{ID: "album-nevermind", Name: "Nevermind", Year: "1991", Type: "Album", ImageURL: artNevermind},
			},
		},
	}
}

func buildAlbums() map[string]*source.AlbumPage {
	return map[string]*source.AlbumPage{
		"album-dsotm": {
			ID: "album-dsotm", Name: "The Dark Side of the Moon", Artist: "Pink Floyd", Year: "1973", ImageURL: artDarkSide,
			Tracks: []source.Track{
				track("t33", "Speak to Me", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 1*time.Minute+30*time.Second),
				track("t25", "Breathe (In the Air)", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 2*time.Minute+43*time.Second),
				track("t34", "On the Run", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 3*time.Minute+36*time.Second),
				track("t09", "Time", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 7*time.Minute+6*time.Second),
				track("t35", "The Great Gig in the Sky", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 4*time.Minute+47*time.Second),
				track("t13", "Money", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 6*time.Minute+22*time.Second),
				track("t36", "Us and Them", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 7*time.Minute+51*time.Second),
				track("t37", "Any Colour You Like", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 3*time.Minute+26*time.Second),
				track("t38", "Brain Damage", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 3*time.Minute+50*time.Second),
				track("t39", "Eclipse", "Pink Floyd", "The Dark Side of the Moon", artDarkSide, "artist-pf", "album-dsotm", 2*time.Minute+6*time.Second),
			},
		},
		"album-rumours": {
			ID: "album-rumours", Name: "Rumours", Artist: "Fleetwood Mac", Year: "1977", ImageURL: artRumours,
			Tracks: []source.Track{
				track("t40", "Second Hand News", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 2*time.Minute+56*time.Second),
				track("t02", "Dreams", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+14*time.Second),
				track("t41", "Never Going Back Again", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 2*time.Minute+14*time.Second),
				track("t42", "Don't Stop", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+11*time.Second),
				track("t10", "Go Your Own Way", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+38*time.Second),
				track("t43", "Songbird", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+20*time.Second),
				track("t06", "The Chain", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+28*time.Second),
				track("t44", "You Make Loving Fun", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+31*time.Second),
				track("t45", "I Don't Want to Know", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+15*time.Second),
				track("t46", "Oh Daddy", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 3*time.Minute+56*time.Second),
				track("t47", "Gold Dust Woman", "Fleetwood Mac", "Rumours", artRumours, "artist-fm", "album-rumours", 4*time.Minute+56*time.Second),
			},
		},
		"album-okc": {
			ID: "album-okc", Name: "OK Computer", Artist: "Radiohead", Year: "1997", ImageURL: artOKComp,
			Tracks: []source.Track{
				track("t48", "Airbag", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+44*time.Second),
				track("t03", "Paranoid Android", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 6*time.Minute+23*time.Second),
				track("t49", "Subterranean Homesick Alien", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+27*time.Second),
				track("t50", "Exit Music (For a Film)", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+24*time.Second),
				track("t51", "Let Down", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+59*time.Second),
				track("t07", "Karma Police", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+21*time.Second),
				track("t52", "Fitter Happier", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 1*time.Minute+57*time.Second),
				track("t53", "Electioneering", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 3*time.Minute+50*time.Second),
				track("t54", "Climbing Up the Walls", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+45*time.Second),
				track("t15", "No Surprises", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 3*time.Minute+48*time.Second),
				track("t11", "Lucky", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 4*time.Minute+19*time.Second),
				track("t55", "The Tourist", "Radiohead", "OK Computer", artOKComp, "artist-rh", "album-okc", 5*time.Minute+24*time.Second),
			},
		},
		"album-kob": {
			ID: "album-kob", Name: "Kind of Blue", Artist: "Miles Davis", Year: "1959", ImageURL: artKindBlue,
			Tracks: []source.Track{
				track("t04", "So What", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+22*time.Second),
				track("t12", "Freddie Freeloader", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+46*time.Second),
				track("t08", "Blue in Green", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 5*time.Minute+27*time.Second),
				track("t28", "All Blues", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 11*time.Minute+33*time.Second),
				track("t29", "Flamenco Sketches", "Miles Davis", "Kind of Blue", artKindBlue, "artist-md", "album-kob", 9*time.Minute+26*time.Second),
			},
		},
		"album-currents": {
			ID: "album-currents", Name: "Currents", Artist: "Tame Impala", Year: "2015", ImageURL: artCurrents,
			Tracks: []source.Track{
				track("t56", "Let It Happen", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 7*time.Minute+47*time.Second),
				track("t65", "Nangs", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 1*time.Minute+47*time.Second),
				track("t66", "The Moment", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 4*time.Minute+15*time.Second),
				track("t67", "Yes I'm Changing", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 4*time.Minute+30*time.Second),
				track("t68", "Eventually", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 5*time.Minute+18*time.Second),
				track("t69", "Gossip", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 0*time.Minute+55*time.Second),
				track("t57", "The Less I Know The Better", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 3*time.Minute+36*time.Second),
				track("t70", "Past Life", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 3*time.Minute+48*time.Second),
				track("t71", "Disciples", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 1*time.Minute+48*time.Second),
				track("t72", "'Cause I'm A Man", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 4*time.Minute+1*time.Second),
				track("t73", "Reality In Motion", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 4*time.Minute+12*time.Second),
				track("t74", "Love/Paranoia", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 3*time.Minute+5*time.Second),
				track("t58", "New Person, Same Old Mistakes", "Tame Impala", "Currents", artCurrents, "artist-ti", "album-currents", 6*time.Minute+3*time.Second),
			},
		},
		"album-pr": {
			ID: "album-pr", Name: "Purple Rain", Artist: "Prince", Year: "1984", ImageURL: artPurpleRain,
			Tracks: []source.Track{
				track("t75", "Let's Go Crazy", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 4*time.Minute+39*time.Second),
				track("t76", "Take Me with U", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 3*time.Minute+54*time.Second),
				track("t77", "The Beautiful Ones", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 5*time.Minute+13*time.Second),
				track("t78", "Computer Blue", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 3*time.Minute+59*time.Second),
				track("t79", "Darling Nikki", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 4*time.Minute+14*time.Second),
				track("t62", "When Doves Cry", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 5*time.Minute+54*time.Second),
				track("t80", "I Would Die 4 U", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 2*time.Minute+49*time.Second),
				track("t81", "Baby I'm a Star", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 4*time.Minute+24*time.Second),
				track("t59", "Purple Rain", "Prince", "Purple Rain", artPurpleRain, "artist-pr", "album-pr", 8*time.Minute+41*time.Second),
			},
		},
		"album-ram": {
			ID: "album-ram", Name: "Random Access Memories", Artist: "Daft Punk", Year: "2013", ImageURL: artRAM,
			Tracks: []source.Track{
				track("t82", "Give Life Back to Music", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 4*time.Minute+35*time.Second),
				track("t83", "The Game of Love", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 5*time.Minute+22*time.Second),
				track("t84", "Giorgio by Moroder", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 9*time.Minute+4*time.Second),
				track("t85", "Within", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 3*time.Minute+48*time.Second),
				track("t64", "Instant Crush", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 5*time.Minute+37*time.Second),
				track("t86", "Lose Yourself to Dance", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 5*time.Minute+53*time.Second),
				track("t87", "Touch", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 8*time.Minute+18*time.Second),
				track("t60", "Get Lucky", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 6*time.Minute+9*time.Second),
				track("t88", "Beyond", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 4*time.Minute+50*time.Second),
				track("t89", "Motherboard", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 5*time.Minute+41*time.Second),
				track("t90", "Fragments of Time", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 4*time.Minute+39*time.Second),
				track("t91", "Doin' it Right", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 4*time.Minute+11*time.Second),
				track("t92", "Contact", "Daft Punk", "Random Access Memories", artRAM, "artist-dp", "album-ram", 6*time.Minute+23*time.Second),
			},
		},
		"album-nevermind": {
			ID: "album-nevermind", Name: "Nevermind", Artist: "Nirvana", Year: "1991", ImageURL: artNevermind,
			Tracks: []source.Track{
				track("t61", "Smells Like Teen Spirit", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 5*time.Minute+1*time.Second),
				track("t93", "In Bloom", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 4*time.Minute+15*time.Second),
				track("t63", "Come As You Are", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+38*time.Second),
				track("t94", "Breed", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+4*time.Second),
				track("t95", "Lithium", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 4*time.Minute+17*time.Second),
				track("t96", "Polly", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 2*time.Minute+53*time.Second),
				track("t97", "Territorial Pissings", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 2*time.Minute+22*time.Second),
				track("t98", "Drain You", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+43*time.Second),
				track("t99", "Lounge Act", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 2*time.Minute+36*time.Second),
				track("t100", "Stay Away", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+31*time.Second),
				track("t101", "On A Plain", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+14*time.Second),
				track("t102", "Something In The Way", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 3*time.Minute+52*time.Second),
				track("t103", "Endless, Nameless", "Nirvana", "Nevermind", artNevermind, "artist-nv", "album-nevermind", 6*time.Minute+43*time.Second),
			},
		},
	}
}

func buildDevices() []source.Device {
	return []source.Device{
		{ID: "dev-1", Name: "MacBook Pro", Type: "Computer", IsActive: true},
		{ID: "dev-2", Name: "Living Room Speaker", Type: "Speaker", IsActive: false},
		{ID: "dev-3", Name: "iPhone", Type: "Smartphone", IsActive: false},
	}
}
