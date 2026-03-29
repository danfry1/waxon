package mood

import "testing"

func TestAllMoodsDefined(t *testing.T) {
	moods := []Mood{Warm, Electric, Drift, Dark, Golden, Bright, Idle}
	for _, m := range moods {
		t.Run(m.Name, func(t *testing.T) {
			if m.Name == "" { t.Error("Name is empty") }
			if m.Primary == "" { t.Error("Primary color is empty") }
			if m.Secondary == "" { t.Error("Secondary color is empty") }
			if m.Background == "" { t.Error("Background color is empty") }
			if m.PatternChar == "" { t.Error("PatternChar is empty") }
			if m.Energy < 0 || m.Energy > 1 { t.Errorf("Energy = %f, want 0.0-1.0", m.Energy) }
		})
	}
}

func TestMoodByName(t *testing.T) {
	got, ok := ByName("electric")
	if !ok { t.Fatal("expected to find 'electric'") }
	if got.Name != "electric" { t.Errorf("Name = %q, want %q", got.Name, "electric") }
	_, ok = ByName("nonexistent")
	if ok { t.Error("expected 'nonexistent' to not be found") }
}
