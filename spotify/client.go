package spotify

import (
	"context"
	"net/http"
	"time"

	"github.com/danielfry/waxon/auth"
	spotifyapi "github.com/zmb3/spotify/v2"
	"golang.org/x/oauth2"
)

// ClientPair holds both the zmb3 spotify client and the raw HTTP client
// (needed because the Spotify API changed some response keys that the
// zmb3 library hasn't updated for).
type ClientPair struct {
	Spotify *spotifyapi.Client
	HTTP    *http.Client
}

func NewClient(clientID string, token *oauth2.Token, tokenPath string) ClientPair {
	cfg := &oauth2.Config{
		ClientID: clientID,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://accounts.spotify.com/authorize",
			TokenURL: "https://accounts.spotify.com/api/token",
		},
	}

	// Inject an HTTP client with timeout into the context so oauth2 token
	// refresh requests don't hang indefinitely on a slow auth server.
	refreshClient := &http.Client{Timeout: 15 * time.Second}
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, refreshClient)

	// Wrap the token source so refreshed tokens are saved to disk
	baseSource := cfg.TokenSource(ctx, token)
	persistSource := auth.NewPersistingTokenSource(baseSource, tokenPath, token)
	httpClient := oauth2.NewClient(context.Background(), persistSource)
	httpClient.Timeout = 15 * time.Second

	client := spotifyapi.New(httpClient)
	return ClientPair{Spotify: client, HTTP: httpClient}
}
