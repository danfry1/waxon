package spotify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/danfry1/waxon/lyrics"
	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

// fullTrackToSource converts a zmb3 FullTrack to a source.Track.
func fullTrackToSource(item spotifyapi.FullTrack) source.Track {
	artist := ""
	artistID := ""
	if len(item.Artists) > 0 {
		artist = item.Artists[0].Name
		artistID = string(item.Artists[0].ID)
	}
	artworkURL := pickImageURL(zmb3Images(item.Album.Images), trackArtMinPx)
	return source.Track{
		ID:         string(item.ID),
		URI:        string(item.URI),
		Name:       item.Name,
		Artist:     artist,
		Album:      item.Album.Name,
		ArtworkURL: artworkURL,
		Duration:   time.Duration(item.Duration) * time.Millisecond,
		ArtistID:   artistID,
		AlbumID:    string(item.Album.ID),
	}
}

type PlayerSource struct {
	client     *spotifyapi.Client
	httpClient *http.Client
	lyrics     *lyrics.Client

	userMu sync.Mutex
	userID string // cached current user ID (for playlist ownership/creation)
}

func NewPlayerSource(cp ClientPair) *PlayerSource {
	return &PlayerSource{
		client:     cp.Spotify,
		httpClient: cp.HTTP,
		lyrics:     lyrics.New(),
	}
}

// apiPlayerState mirrors the fields of GET /me/player that waxon uses.
type apiPlayerState struct {
	Device struct {
		Name          string `json:"name"`
		VolumePercent int    `json:"volume_percent"`
	} `json:"device"`
	RepeatState  string `json:"repeat_state"`
	ShuffleState bool   `json:"shuffle_state"`
	Context      *struct {
		URI string `json:"uri"`
	} `json:"context"`
	ProgressMS int64 `json:"progress_ms"`
	IsPlaying  bool  `json:"is_playing"`
	// CurrentlyPlayingType is "track", "episode", "ad" or "unknown".
	CurrentlyPlayingType string          `json:"currently_playing_type"`
	Item                 json.RawMessage `json:"item"` // a track or, with additional_types=episode, an episode
}

// CurrentPlayback reads GET /me/player through waxon's own API path (rather
// than the zmb3 client) so rate-limit responses keep their Retry-After and
// 204 "nothing playing" is distinguished from errors.
func (p *PlayerSource) CurrentPlayback(ctx context.Context) (*source.PlaybackState, error) {
	var state apiPlayerState
	found, err := p.apiGetOptional(ctx, "/me/player?additional_types=episode", &state)
	if err != nil {
		return nil, fmt.Errorf("player state: %w", err)
	}
	if !found || len(state.Item) == 0 || string(state.Item) == "null" {
		return nil, nil
	}
	track, ok := playingItem(state.CurrentlyPlayingType, state.Item)
	if !ok {
		return nil, nil // an ad, or nothing we can show
	}
	track.Position = time.Duration(state.ProgressMS) * time.Millisecond
	track.Playing = state.IsPlaying
	track.DeviceName = state.Device.Name

	repeatMode := source.RepeatOff
	switch state.RepeatState {
	case "context":
		repeatMode = source.RepeatContext
	case "track":
		repeatMode = source.RepeatTrack
	}
	contextURI := ""
	if state.Context != nil {
		contextURI = state.Context.URI
	}
	return &source.PlaybackState{
		Track:      &track,
		Volume:     state.Device.VolumePercent,
		ShuffleOn:  state.ShuffleState,
		RepeatMode: repeatMode,
		ContextURI: contextURI,
	}, nil
}

func (p *PlayerSource) Play(ctx context.Context) error {
	return wrapPlayerError(p.client.Play(ctx))
}

func (p *PlayerSource) Pause(ctx context.Context) error {
	return wrapPlayerError(p.client.Pause(ctx))
}

func (p *PlayerSource) Next(ctx context.Context) error {
	return wrapPlayerError(p.client.Next(ctx))
}

func (p *PlayerSource) Previous(ctx context.Context) error {
	return wrapPlayerError(p.client.Previous(ctx))
}

func (p *PlayerSource) Seek(ctx context.Context, position time.Duration) error {
	ms := int(position.Milliseconds())
	return wrapPlayerError(p.client.Seek(ctx, ms))
}

func (p *PlayerSource) SetVolume(ctx context.Context, percent int) error {
	return wrapPlayerError(p.client.Volume(ctx, percent))
}

func (p *PlayerSource) SetShuffle(ctx context.Context, state bool) error {
	return wrapPlayerError(p.client.Shuffle(ctx, state))
}

func (p *PlayerSource) SetRepeat(ctx context.Context, mode source.RepeatMode) error {
	return wrapPlayerError(p.client.Repeat(ctx, string(mode)))
}

func (p *PlayerSource) Devices(ctx context.Context) ([]source.Device, error) {
	devs, err := p.client.PlayerDevices(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]source.Device, len(devs))
	for i, d := range devs {
		result[i] = source.Device{
			ID:       string(d.ID),
			Name:     d.Name,
			Type:     d.Type,
			IsActive: d.Active,
		}
	}
	return result, nil
}

func (p *PlayerSource) TransferPlayback(ctx context.Context, deviceID string) error {
	id := spotifyapi.ID(deviceID)
	err := wrapPlayerError(p.client.TransferPlayback(ctx, id, true))
	if errors.Is(err, source.ErrDeviceUnavailable) || errors.Is(err, source.ErrPlaybackRestricted) {
		// Spotify refuses a handover it can't make: a server error from
		// librespot to the Spotify app, "Restriction violated" when the
		// device playing has gone (a Spotify app that was quit). Starting
		// the same track at the same point on the device does the same
		// thing without the handover.
		playErr := p.playHere(ctx, id)
		slog.Info("transfer refused; played on the device instead", "device", deviceID, "transfer", err, "play", playErr)
		if playErr == nil || !errors.Is(playErr, errNothingPlaying) {
			return playErr
		}
	}
	return err
}

var errNothingPlaying = errors.New("nothing playing to move")

// playHere plays what's playing now on device id: the same track, in the
// same playlist or album, from the same point.
func (p *PlayerSource) playHere(ctx context.Context, id spotifyapi.ID) error {
	// Episodes are reported only when asked for (and then decode into Item
	// all the same: its uri is what's needed).
	st, err := p.client.PlayerState(ctx, spotifyapi.AdditionalTypes(spotifyapi.EpisodeAdditionalType))
	if err != nil {
		return wrapPlayerError(err)
	}
	if st == nil || st.Item == nil || st.Item.URI == "" {
		return errNothingPlaying
	}
	// Stop the device playing first, or both play for a moment: Spotify
	// doesn't stop it, as a real handover would. A failed pause is no
	// reason not to move.
	if from := st.Device.ID; from != "" && from != id && st.Playing {
		_ = p.client.PauseOpt(ctx, &spotifyapi.PlayOptions{DeviceID: &from})
	}
	opt := &spotifyapi.PlayOptions{DeviceID: &id, PositionMs: st.Progress}
	switch uri := spotifyapi.URI(p.playableContext(ctx, string(st.PlaybackContext.URI))); {
	case offsetContext(string(uri)):
		opt.PlaybackContext = &uri
		opt.PlaybackOffset = &spotifyapi.PlaybackOffset{URI: st.Item.URI}
	default:
		// An artist, show or radio context can't be started at a given
		// track; the track itself plays on.
		opt.URIs = []spotifyapi.URI{st.Item.URI}
	}
	return wrapPlayerError(p.client.PlayOpt(ctx, opt))
}

// Queue reads the queue through waxon's own API path so episodes keep their
// type (zmb3 decodes every entry as a track).
func (p *PlayerSource) Queue(ctx context.Context) ([]source.Track, error) {
	var q struct {
		Queue []json.RawMessage `json:"queue"`
	}
	if err := p.apiGet(ctx, "/me/player/queue", &q); err != nil {
		return nil, err
	}
	tracks := make([]source.Track, 0, len(q.Queue))
	for _, raw := range q.Queue {
		var kind struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(raw, &kind)
		if t, ok := playingItem(kind.Type, raw); ok {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}

func (p *PlayerSource) PlayTrack(ctx context.Context, contextURI string, trackURI string) error {
	uri := spotifyapi.URI(p.playableContext(ctx, contextURI))
	opts := &spotifyapi.PlayOptions{
		PlaybackContext: &uri,
	}
	if trackURI != "" {
		opts.PlaybackOffset = &spotifyapi.PlaybackOffset{URI: spotifyapi.URI(trackURI)}
	}
	return wrapPlayerError(p.client.PlayOpt(ctx, opts))
}

// offsetContext reports whether playback in context uri can start at a
// given track (Spotify allows that for albums, playlists and the user's
// collection only).
func offsetContext(uri string) bool {
	parts := strings.Split(uri, ":")
	switch {
	case len(parts) >= 3 && (parts[1] == "album" || parts[1] == "playlist"):
		return true
	case len(parts) == 4 && parts[1] == "user" && parts[3] == "collection":
		return true
	}
	return false
}

// likedSongsURI is how waxon names Liked Songs (source.Playlist.URI).
const likedSongsURI = "spotify:collection:tracks"

// playableContext is contextURI as a playback request should name it. The
// Spotify apps refuse Liked Songs as spotify:collection:tracks ("Restriction
// violated"; librespot accepts it) but play it as the account's collection,
// spotify:user:<id>:collection.
func (p *PlayerSource) playableContext(ctx context.Context, contextURI string) string {
	if contextURI != likedSongsURI {
		return contextURI
	}
	id, err := p.currentUserID(ctx)
	if err != nil || id == "" {
		return contextURI
	}
	return "spotify:user:" + id + ":collection"
}

func (p *PlayerSource) PlayTrackDirect(ctx context.Context, trackURI string) error {
	opts := &spotifyapi.PlayOptions{
		URIs: []spotifyapi.URI{spotifyapi.URI(trackURI)},
	}
	return wrapPlayerError(p.client.PlayOpt(ctx, opts))
}

func (p *PlayerSource) AddToQueue(ctx context.Context, trackID string) error {
	if isURI(trackID) { // an episode: zmb3's QueueSong only builds track URIs
		return wrapPlayerError(p.apiWrite(ctx, http.MethodPost, "/me/player/queue?uri="+url.QueryEscape(trackID), nil, nil))
	}
	return wrapPlayerError(p.client.QueueSong(ctx, spotifyapi.ID(trackID)))
}

func (p *PlayerSource) RecentlyPlayed(ctx context.Context) ([]source.Track, error) {
	var page struct {
		Items []struct {
			Track apiTrack `json:"track"`
		} `json:"items"`
	}
	if err := p.apiGet(ctx, "/me/player/recently-played?limit=50", &page); err != nil {
		return nil, fmt.Errorf("recently played: %w", err)
	}
	tracks := make([]source.Track, 0, len(page.Items))
	for _, item := range page.Items {
		tracks = append(tracks, apiTrackToSource(item.Track))
	}
	return tracks, nil
}

func (p *PlayerSource) Lyrics(ctx context.Context, track source.Track) (*source.Lyrics, error) {
	return p.lyrics.Get(ctx, track)
}

// currentUserID returns the authenticated user's Spotify ID, fetched once.
func (p *PlayerSource) currentUserID(ctx context.Context) (string, error) {
	p.userMu.Lock()
	defer p.userMu.Unlock()
	if p.userID != "" {
		return p.userID, nil
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := p.apiGet(ctx, "/me", &me); err != nil {
		return "", err
	}
	p.userID = me.ID
	return p.userID, nil
}

// Profile returns the signed-in user's ID and display name (for
// `waxon doctor`).
func (p *PlayerSource) Profile(ctx context.Context) (id, name string, err error) {
	var me struct {
		ID          string `json:"id"`
		DisplayName string `json:"display_name"`
	}
	if err := p.apiGet(ctx, "/me", &me); err != nil {
		return "", "", err
	}
	return me.ID, me.DisplayName, nil
}

// --- source.PlaylistSource ---

func (p *PlayerSource) AddToPlaylist(ctx context.Context, playlistID, trackID string) error {
	body := map[string]any{"uris": []string{itemURI(trackID)}}
	err := p.apiWrite(ctx, http.MethodPost, "/playlists/"+playlistID+"/items", body, nil)
	if err != nil && isLegacyFallbackError(err) && !isURI(trackID) {
		_, err = p.client.AddTracksToPlaylist(ctx, spotifyapi.ID(playlistID), spotifyapi.ID(trackID))
	}
	return wrapScopeError(err)
}

func (p *PlayerSource) RemoveFromPlaylist(ctx context.Context, playlistID, trackID string, position int) error {
	item := map[string]any{"uri": itemURI(trackID)}
	if position >= 0 {
		// Position-based removal touches just the selected row, so duplicates
		// of the same track elsewhere in the playlist are left alone.
		item["positions"] = []int{position}
	}
	body := map[string]any{"items": []map[string]any{item}}
	err := p.apiWrite(ctx, http.MethodDelete, "/playlists/"+playlistID+"/items", body, nil)
	if err != nil && isLegacyFallbackError(err) && (position >= 0 || !isURI(trackID)) {
		if position < 0 {
			_, err = p.client.RemoveTracksFromPlaylist(ctx, spotifyapi.ID(playlistID), spotifyapi.ID(trackID))
		} else {
			tracks := []spotifyapi.TrackToRemove{{URI: itemURI(trackID), Positions: []int{position}}}
			_, err = p.client.RemoveTracksFromPlaylistOpt(ctx, spotifyapi.ID(playlistID), tracks, "")
		}
	}
	return wrapScopeError(err)
}

func (p *PlayerSource) CreatePlaylist(ctx context.Context, name string) (source.Playlist, error) {
	var created struct {
		ID   string `json:"id"`
		URI  string `json:"uri"`
		Name string `json:"name"`
	}
	body := map[string]any{"name": name, "public": false}
	err := p.apiWrite(ctx, http.MethodPost, "/me/playlists", body, &created)
	if err != nil && isLegacyFallbackError(err) {
		me, merr := p.currentUserID(ctx)
		if merr != nil {
			return source.Playlist{}, wrapScopeError(merr)
		}
		pl, cerr := p.client.CreatePlaylistForUser(ctx, me, name, "", false, false)
		if cerr != nil {
			return source.Playlist{}, wrapScopeError(cerr)
		}
		created.ID, created.URI, created.Name = string(pl.ID), string(pl.URI), pl.Name
		err = nil
	}
	if err != nil {
		return source.Playlist{}, wrapScopeError(err)
	}
	return source.Playlist{ID: created.ID, URI: created.URI, Name: created.Name, Editable: true}, nil
}
