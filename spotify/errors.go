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
	case sErr.Status == http.StatusForbidden && strings.Contains(msg, "scope"):
		return fmt.Errorf("%w: %w", source.ErrInsufficientScope, err)
	}
	return err
}

// wrapScopeError maps 403s from non-player endpoints: "Insufficient client
// scope" becomes source.ErrInsufficientScope (re-auth fixes it); any other
// 403 becomes source.ErrForbidden (the endpoint isn't available to this app).
func wrapScopeError(err error) error {
	if err == nil {
		return nil
	}
	var sErr spotifyapi.Error
	if errors.As(err, &sErr) && sErr.Status == http.StatusForbidden {
		if strings.Contains(strings.ToLower(sErr.Message), "scope") {
			return fmt.Errorf("%w: %w", source.ErrInsufficientScope, err)
		}
		return fmt.Errorf("%w: %w", source.ErrForbidden, err)
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusForbidden {
		if strings.Contains(strings.ToLower(apiErr.Body), "scope") {
			return fmt.Errorf("%w: %w", source.ErrInsufficientScope, err)
		}
		return fmt.Errorf("%w: %w", source.ErrForbidden, err)
	}
	return err
}
