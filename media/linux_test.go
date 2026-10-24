//go:build linux

package media

import (
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestMPRISMetadata(t *testing.T) {
	md := mprisMetadata(State{Title: "Song", Artist: "Band", Album: "LP", Duration: 3 * time.Minute, ArtworkURL: "https://img/a"})
	if md["xesam:title"].Value() != "Song" || md["xesam:album"].Value() != "LP" {
		t.Errorf("metadata = %v", md)
	}
	if a, ok := md["xesam:artist"].Value().([]string); !ok || len(a) != 1 || a[0] != "Band" {
		t.Errorf("artist = %v (MPRIS wants a list)", md["xesam:artist"].Value())
	}
	if md["mpris:length"].Value() != int64(180_000_000) {
		t.Errorf("length = %v, want microseconds", md["mpris:length"].Value())
	}
	if md["mpris:trackid"].Value() != mprisTrack || md["mpris:artUrl"].Value() != "https://img/a" {
		t.Errorf("trackid/artUrl = %v / %v", md["mpris:trackid"].Value(), md["mpris:artUrl"].Value())
	}
	if len(mprisMetadata(State{Empty: true})) != 0 {
		t.Error("nothing playing should have empty metadata")
	}
	var _ dbus.ObjectPath = mprisTrack
}
