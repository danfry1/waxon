package mood

import (
	"strings"

	"github.com/danielfry/spotui/source"
)

var artistMoods = map[string]Mood{
	"bon iver": Warm, "iron & wine": Warm, "fleet foxes": Warm, "sufjan stevens": Warm,
	"jose gonzalez": Warm, "nick drake": Warm, "elliott smith": Warm, "phoebe bridgers": Warm,
	"big thief": Warm, "the lumineers": Warm,

	"m83": Electric, "daft punk": Electric, "the weeknd": Electric, "deadmau5": Electric,
	"justice": Electric, "lcd soundsystem": Electric, "tame impala": Electric, "charli xcx": Electric,
	"disclosure": Electric, "flume": Electric,

	"brian eno": Drift, "tycho": Drift, "boards of canada": Drift, "aphex twin": Drift,
	"nils frahm": Drift, "olafur arnalds": Drift, "sigur ros": Drift, "bonobo": Drift,
	"khruangbin": Drift,

	"nine inch nails": Dark, "tool": Dark, "radiohead": Dark, "massive attack": Dark,
	"portishead": Dark, "nick cave": Dark, "depeche mode": Dark, "type o negative": Dark,
	"black sabbath": Dark,

	"miles davis": Golden, "john coltrane": Golden, "nina simone": Golden, "erykah badu": Golden,
	"d'angelo": Golden, "bill evans": Golden, "amy winehouse": Golden, "anderson .paak": Golden,
	"sade": Golden, "frank ocean": Golden,

	"haim": Bright, "paramore": Bright, "carly rae jepsen": Bright, "chappell roan": Bright,
	"bleachers": Bright, "the 1975": Bright, "walk the moon": Bright, "passion pit": Bright,
}

var keywordMoods = []struct {
	keywords []string
	mood     Mood
}{
	{[]string{"acoustic", "unplugged", "folk", "campfire"}, Warm},
	{[]string{"jazz", "soul", "funk", "groove", "swing", "blues"}, Golden},
	{[]string{"remix", "edm", "techno", "synth", "electronic", "club"}, Electric},
	{[]string{"chill", "ambient", "lo-fi", "lofi", "sleep", "relax", "meditation"}, Drift},
	{[]string{"metal", "heavy", "doom", "dark", "goth", "industrial"}, Dark},
	{[]string{"pop", "bright", "happy", "sunshine", "summer", "dance"}, Bright},
}

func DetectMood(artist, track, album string) Mood {
	artistLower := strings.ToLower(artist)
	if m, ok := artistMoods[artistLower]; ok {
		return m
	}
	combined := strings.ToLower(artist + " " + track + " " + album)
	for _, km := range keywordMoods {
		for _, kw := range km.keywords {
			if strings.Contains(combined, kw) {
				return km.mood
			}
		}
	}
	return Warm
}

// DetectFromFeatures uses Spotify audio features for more accurate mood detection.
func DetectFromFeatures(f *source.AudioFeatures) Mood {
	if f == nil {
		return Warm
	}

	type scored struct {
		mood  Mood
		score float64
	}

	candidates := []scored{
		{Electric, scoreElectric(f)},
		{Dark, scoreDark(f)},
		{Bright, scoreBright(f)},
		{Golden, scoreGolden(f)},
		{Drift, scoreDrift(f)},
		{Warm, scoreWarm(f)},
	}

	best := candidates[0]
	for _, c := range candidates[1:] {
		if c.score > best.score {
			best = c
		}
	}
	return best.mood
}

func scoreElectric(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy > 0.7 && f.Valence >= 0.3 {
		score += f.Energy
	}
	if f.Danceability > 0.6 {
		score += f.Danceability * 0.5
	}
	return score
}

func scoreDark(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy > 0.5 && f.Valence < 0.3 {
		score += f.Energy * (1 - f.Valence)
	}
	return score
}

func scoreBright(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Valence > 0.6 && f.Energy > 0.5 {
		score += f.Valence * 0.6
		if f.Danceability > 0.5 {
			score += 0.3
		}
	}
	return score
}

func scoreGolden(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy > 0.3 && f.Energy < 0.6 && f.Valence > 0.4 && f.Valence < 0.8 {
		score += 0.7
		if f.Danceability > 0.4 && f.Danceability < 0.7 {
			score += 0.3
		}
	}
	return score
}

func scoreDrift(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Energy < 0.3 {
		score += (1 - f.Energy)
		if f.Acousticness > 0.3 {
			score += f.Acousticness * 0.3
		}
	}
	return score
}

func scoreWarm(f *source.AudioFeatures) float64 {
	score := 0.0
	if f.Acousticness > 0.5 {
		score += f.Acousticness * 0.5
	}
	if f.Energy > 0.2 && f.Energy < 0.5 {
		score += 0.4
	}
	return score
}
