package mood

import "testing"

func TestTransitionStart(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	if !tr.Active {
		t.Error("expected transition to be active")
	}
	if tr.Progress() != 0.0 {
		t.Errorf("initial progress = %f, want 0.0", tr.Progress())
	}
}

func TestTransitionTick(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	for range 200 {
		tr.Tick()
	}
	if tr.Progress() < 0.95 {
		t.Errorf("after 200 ticks, progress = %f, want >= 0.95", tr.Progress())
	}
}

func TestTransitionDone(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	for range 500 {
		tr.Tick()
	}
	if !tr.Done() {
		t.Error("expected transition to be done after 500 ticks")
	}
}

func TestTransitionCurrentMood(t *testing.T) {
	tr := NewTransition(Warm, Electric)
	cur := tr.Current()
	if cur.Name != "warm → electric" {
		t.Errorf("Name = %q, want %q", cur.Name, "warm → electric")
	}
	if cur.Energy < Warm.Energy-0.01 || cur.Energy > Warm.Energy+0.01 {
		t.Errorf("initial energy = %f, want ~%f", cur.Energy, Warm.Energy)
	}
}
