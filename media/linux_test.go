//go:build linux

package media

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestMPRISMetadata(t *testing.T) {
	md := mprisMetadata(State{ID: "4uLU6hMCjMI75M1A2tKUQC", Title: "Song", Artist: "Band", Album: "LP", Duration: 3 * time.Minute, ArtworkURL: "https://img/a"})
	if md["xesam:title"].Value() != "Song" || md["xesam:album"].Value() != "LP" {
		t.Errorf("metadata = %v", md)
	}
	if a, ok := md["xesam:artist"].Value().([]string); !ok || len(a) != 1 || a[0] != "Band" {
		t.Errorf("artist = %v (MPRIS wants a list)", md["xesam:artist"].Value())
	}
	if md["mpris:length"].Value() != int64(180_000_000) {
		t.Errorf("length = %v, want microseconds", md["mpris:length"].Value())
	}
	if md["mpris:trackid"].Value() != dbus.ObjectPath("/org/waxon/track/4uLU6hMCjMI75M1A2tKUQC") || md["mpris:artUrl"].Value() != "https://img/a" {
		t.Errorf("trackid/artUrl = %v / %v", md["mpris:trackid"].Value(), md["mpris:artUrl"].Value())
	}
	if len(mprisMetadata(State{Empty: true})) != 0 {
		t.Error("nothing playing should have empty metadata")
	}
}

// Track ids are valid object paths, distinct per track, even for local
// files (whose id is a URI full of characters paths can't hold).
func TestMPRISTrackID(t *testing.T) {
	local := mprisTrackID(State{ID: "spotify:local:Artist:Album:Song+Name:180"})
	if !local.IsValid() {
		t.Errorf("%q isn't a valid object path", local)
	}
	if mprisTrackID(State{ID: "a"}) == mprisTrackID(State{ID: "b"}) {
		t.Error("two tracks share an id")
	}
	if mprisTrackID(State{Empty: true}) != mprisNoTrack {
		t.Error("nothing playing should be NoTrack")
	}
}
