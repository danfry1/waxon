package app

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// readmeConfigList returns the backticked, space-separated list that follows
// marker on the README's "**Keys**" configuration line.
func readmeConfigList(t *testing.T, readme, marker string) []string {
	t.Helper()
	for _, line := range strings.Split(readme, "\n") {
		if !strings.HasPrefix(line, "**Keys**") {
			continue
		}
		_, rest, ok := strings.Cut(line, marker+" (`")
		if !ok {
			_, rest, ok = strings.Cut(line, marker+" `")
		}
		if !ok {
			t.Fatalf("README **Keys** line has no %q list", marker)
		}
		list, _, _ := strings.Cut(rest, "`")
		return strings.Fields(list)
	}
	t.Fatal("README has no **Keys** configuration line")
	return nil
}

// TestReadmeMatchesKeymap keeps the README's key, g-motion and command
// references in step with the code, as the help overlay already is.
func TestReadmeMatchesKeymap(t *testing.T) {
	b, err := os.ReadFile("../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := strings.ReplaceAll(string(b), "\r\n", "\n") // Windows checkouts

	for _, g := range gMotions {
		if !strings.Contains(readme, "`g"+g.Key+"`") {
			t.Errorf("README does not document the g%s motion (%s)", g.Key, g.Desc)
		}
	}

	var motions []string
	for _, g := range gMotions {
		motions = append(motions, "g"+g.Key)
	}
	if got := readmeConfigList(t, readme, "motions"); !slices.Equal(got, motions) {
		t.Errorf("README lists fixed g-motions %v, code has %v", got, motions)
	}

	got := readmeConfigList(t, readme, "Actions:")
	slices.Sort(got)
	if want := KeyActionNames(); !slices.Equal(got, want) {
		t.Errorf("README lists configurable actions %v, code has %v", got, want)
	}

	_, commands, _ := strings.Cut(readme, "\n## Commands\n")
	commands, _, _ = strings.Cut(commands, "\n## ")
	for _, c := range commandHelp {
		name := strings.Fields(c.Key)[0]
		if !strings.Contains(commands, "| `"+name) {
			t.Errorf("README Commands table is missing %s", name)
		}
	}
}
