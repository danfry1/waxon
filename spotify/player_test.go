package spotify

import (
	"testing"

	"github.com/danielfry/spotui/source"
)

var _ source.TrackSource = (*PlayerSource)(nil)
var _ source.RichSource = (*PlayerSource)(nil)

func TestNewPlayerSource(t *testing.T) {
	t.Log("PlayerSource satisfies TrackSource and RichSource interfaces")
}
