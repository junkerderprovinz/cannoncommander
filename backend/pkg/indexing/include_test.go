package indexing

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestAnIncludedFolderDeepInsideTakesPartWithTheWayToIt(t *testing.T) {
	idx := &Index{}
	idx.Config.ResolvedRules = settings.ResolvedRulesConfig{IncludeRootItems: map[string]struct{}{"/home/jdp/": {}}}
	cases := []struct {
		dir, name string
		want      bool
	}{
		{"/", "home", true},
		{"/", "etc", false},
		{"/home/", "jdp", true},
		{"/home/", "anna", false},
		{"/home/", "notes.txt", false},
		{"/home/jdp/", "Bilder", true},
		{"/home/jdp/Bilder/", "strand.jpg", true},
	}
	for _, c := range cases {
		if got := idx.shouldInclude(c.dir, c.name); got != c.want {
			t.Errorf("%s%s: got %v", c.dir, c.name, got)
		}
	}
}

func TestWithoutIncludedItemsEverythingTakesPart(t *testing.T) {
	if !(&Index{}).shouldInclude("/etc/", "hosts") {
		t.Fatal("left out")
	}
}
