//go:build demo

package demo

import (
	"bytes"
	"embed"
	"image"
	_ "image/jpeg"
	"sync"
)

//go:embed art/*.jpg
var artFS embed.FS

var artOnce sync.Once
var artCache map[string]image.Image

// artMapping maps demo:// URLs to embedded file paths.
var artMapping = map[string]string{
	artDarkSide: "art/dark-side-of-the-moon.jpg",
	artRumours:  "art/rumours.jpg",
	artOKComp:   "art/ok-computer.jpg",
	artKindBlue: "art/kind-of-blue.jpg",
	artPLLiked:   "art/liked-songs.jpg",
	artPLRock:    "art/classic-rock.jpg",
	artPLCoding:  "art/late-night-coding.jpg",
	artPLJazz:    "art/jazz-essentials.jpg",
	artCurrents:  "art/currents.jpg",
	artPurpleRain: "art/purple-rain.jpg",
	artRAM:       "art/random-access-memories.jpg",
	artNevermind: "art/nevermind.jpg",
}

func loadArtCache() {
	artOnce.Do(func() {
		artCache = make(map[string]image.Image)
		for url, path := range artMapping {
			data, err := artFS.ReadFile(path)
			if err != nil {
				continue
			}
			img, _, err := image.Decode(bytes.NewReader(data))
			if err != nil {
				continue
			}
			artCache[url] = img
		}
	})
}

// ArtworkImage returns an embedded image for the given demo:// URL.
// Satisfies the app.ArtworkProvider interface.
func (d *DemoSource) ArtworkImage(url string) (image.Image, bool) {
	loadArtCache()
	img, ok := artCache[url]
	return img, ok
}
