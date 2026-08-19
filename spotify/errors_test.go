package spotify

import (
	"errors"
	"fmt"
	"testing"

	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

func TestWrapPlayerError(t *testing.T) {
	other := errors.New("boom")
	cases := []struct {
		name string
		in   error
		want error // sentinel expected via errors.Is, or nil for pass-through
	}{
		{"nil", nil, nil},
		{"non-spotify error passes through", other, nil},
		{"404 no active device", spotifyapi.Error{Status: 404, Message: "Player command failed: No active device found"}, source.ErrNoActiveDevice},
		{"404 any message", spotifyapi.Error{Status: 404, Message: "Device not found"}, source.ErrNoActiveDevice},
		{"403 premium", spotifyapi.Error{Status: 403, Message: "Player command failed: Premium required"}, source.ErrPremiumRequired},
		{"403 other", spotifyapi.Error{Status: 403, Message: "Forbidden"}, nil},
		{"wrapped 404", fmt.Errorf("play: %w", spotifyapi.Error{Status: 404, Message: "x"}), source.ErrNoActiveDevice},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := wrapPlayerError(tc.in)
			if tc.in == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			if tc.want == nil {
				if got != tc.in { //nolint:errorlint // identity check is intended
					t.Fatalf("got %v, want original error unchanged", got)
				}
				return
			}
			if !errors.Is(got, tc.want) {
				t.Fatalf("got %v, want errors.Is %v", got, tc.want)
			}
			// The original Spotify error must stay inspectable.
			var sErr spotifyapi.Error
			if !errors.As(got, &sErr) {
				t.Fatal("wrapped error should still unwrap to spotifyapi.Error")
			}
		})
	}
}
