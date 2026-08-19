package spotify

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/danfry1/waxon/source"
	spotifyapi "github.com/zmb3/spotify/v2"
)

// wrapPlayerError maps Spotify player-endpoint failures onto the source
// package's sentinel errors so the UI can react (activate a device, explain
// the Premium requirement) instead of showing a raw API message.
//
// Spotify's player endpoints answer 404 when there is no active device (the
// documented "NO_ACTIVE_DEVICE" reason) and 403 when the account isn't
// Premium ("PREMIUM_REQUIRED"). The zmb3 client only surfaces the status and
// message, so we match on those. Other errors pass through unchanged.
func wrapPlayerError(err error) error {
	if err == nil {
		return nil
	}
	var sErr spotifyapi.Error
	if !errors.As(err, &sErr) {
		return err
	}
	msg := strings.ToLower(sErr.Message)
	switch {
	case sErr.Status == http.StatusNotFound:
		// Any 404 from a player command means the target device isn't there.
		return fmt.Errorf("%w: %w", source.ErrNoActiveDevice, err)
	case sErr.Status == http.StatusForbidden && strings.Contains(msg, "premium"):
		return fmt.Errorf("%w: %w", source.ErrPremiumRequired, err)
	}
	return err
}
