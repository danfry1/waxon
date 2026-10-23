package spotify

import (
	"context"
	"fmt"

	"github.com/danfry1/waxon/source"
)

// maxLibraryItems caps how much of a saved-albums or followed-artists list is
// fetched. Spotify pages these 50 at a time; a few thousand is plenty for a
// sidebar list and bounds the request count for very large libraries.
const maxLibraryItems = 2000

// SavedAlbums lists the albums in the user's library, most recently saved
// first.
func (p *PlayerSource) SavedAlbums(ctx context.Context) ([]source.SavedAlbum, error) {
	var albums []source.SavedAlbum
	for offset := 0; offset < maxLibraryItems; offset += 50 {
		var page struct {
			Items []struct {
				Album struct {
					ID          string     `json:"id"`
					Name        string     `json:"name"`
					ReleaseDate string     `json:"release_date"`
					Images      []apiImage `json:"images"`
					Artists     []struct {
						Name string `json:"name"`
					} `json:"artists"`
				} `json:"album"`
			} `json:"items"`
			Next string `json:"next"`
		}
		if err := p.apiGet(ctx, fmt.Sprintf("/me/albums?limit=50&offset=%d", offset), &page); err != nil {
			return albums, wrapScopeError(err)
		}
		for _, it := range page.Items {
			a := it.Album
			artist := ""
			if len(a.Artists) > 0 {
				artist = a.Artists[0].Name
			}
			year := a.ReleaseDate
			if len(year) >= 4 {
				year = year[:4]
			}
			albums = append(albums, source.SavedAlbum{
				ID:       a.ID,
				Name:     a.Name,
				Artist:   artist,
				Year:     year,
				ImageURL: pickImageURL(a.Images, thumbArtMinPx),
			})
		}
		if page.Next == "" || len(page.Items) == 0 {
			break
		}
	}
	return albums, nil
}

// FollowedArtists lists the artists the user follows. The endpoint pages by
// cursor (the last artist ID seen), not by offset.
func (p *PlayerSource) FollowedArtists(ctx context.Context) ([]source.SearchArtist, error) {
	var artists []source.SearchArtist
	after := ""
	for len(artists) < maxLibraryItems {
		var page struct {
			Artists struct {
				Items []struct {
					ID     string     `json:"id"`
					Name   string     `json:"name"`
					Images []apiImage `json:"images"`
				} `json:"items"`
				Cursors struct {
					After string `json:"after"`
				} `json:"cursors"`
			} `json:"artists"`
		}
		path := "/me/following?type=artist&limit=50"
		if after != "" {
			path += "&after=" + after
		}
		if err := p.apiGet(ctx, path, &page); err != nil {
			return artists, wrapScopeError(err)
		}
		for _, a := range page.Artists.Items {
			artists = append(artists, source.SearchArtist{
				ID:       a.ID,
				Name:     a.Name,
				ImageURL: pickImageURL(a.Images, thumbArtMinPx),
			})
		}
		after = page.Artists.Cursors.After
		if after == "" || len(page.Artists.Items) == 0 {
			break
		}
	}
	return artists, nil
}
