package mood

type Mood struct {
	Name        string
	Primary     string
	Secondary   string
	Background  string
	PatternChar string
	Energy      float64
}

var (
	Warm     = Mood{Name: "warm", Primary: "#d4a854", Secondary: "#8b7a54", Background: "#1a1510", PatternChar: "~ · ", Energy: 0.3}
	Electric = Mood{Name: "electric", Primary: "#e040fb", Secondary: "#9575cd", Background: "#0d0a1a", PatternChar: "╱╲", Energy: 0.85}
	Drift    = Mood{Name: "drift", Primary: "#26c6da", Secondary: "#4db6ac", Background: "#0a1520", PatternChar: "≋ ~ ", Energy: 0.15}
	Dark     = Mood{Name: "dark", Primary: "#ef5350", Secondary: "#b71c1c", Background: "#1a0a0a", PatternChar: "▪ ▫ ", Energy: 0.65}
	Golden   = Mood{Name: "golden", Primary: "#ffab40", Secondary: "#ff6e40", Background: "#1a1208", PatternChar: "♪ · ♫ · ", Energy: 0.45}
	Bright   = Mood{Name: "bright", Primary: "#ff6b9d", Secondary: "#ffd93d", Background: "#1a1018", PatternChar: "✦ · ", Energy: 0.6}
	Idle     = Mood{Name: "idle", Primary: "#555555", Secondary: "#333333", Background: "#0a0a0a", PatternChar: "· ", Energy: 0.05}
)

var allMoods = []Mood{Warm, Electric, Drift, Dark, Golden, Bright}

func ByName(name string) (Mood, bool) {
	for _, m := range allMoods {
		if m.Name == name {
			return m, true
		}
	}
	if name == "idle" {
		return Idle, true
	}
	return Mood{}, false
}
