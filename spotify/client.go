package spotify

import (
	"context"

	"github.com/danielfry/spotui/auth"
	spotifyapi "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

func NewClient(clientID string, token *oauth2.Token, tokenPath string) *spotifyapi.Client {
	authenticator := spotifyauth.New(
		spotifyauth.WithClientID(clientID),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadPlaybackState,
			spotifyauth.ScopeUserModifyPlaybackState,
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopePlaylistReadPrivate,
			spotifyauth.ScopeUserLibraryRead,
		),
	)

	httpClient := authenticator.Client(context.Background(), token)
	client := spotifyapi.New(httpClient)

	go func() {
		newToken, err := authenticator.RefreshToken(context.Background(), token)
		if err == nil && newToken.AccessToken != token.AccessToken {
			auth.SaveToken(tokenPath, newToken)
		}
	}()

	return client
}
