package mood

import "strings"

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
