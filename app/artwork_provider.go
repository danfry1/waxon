package app

import "image"

// ArtworkProvider is an optional interface that source implementations can
// satisfy to provide embedded artwork instead of HTTP fetching. Used by demo
// mode to avoid network calls.
type ArtworkProvider interface {
	ArtworkImage(url string) (image.Image, bool)
}
