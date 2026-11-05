package accounts

import (
	"fmt"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func bookOf(patterns ...string) *ruleBook {
	b := &ruleBook{groupsOf: func(string) []string { return nil }}
	for _, p := range patterns {
		b.rules = append(b.rules, compile(Rule{Who: "anna", Path: p, Rule: ruleHide}))
	}
	return b
}

const deepPath = "/mnt/user/fotos/2025/urlaub/kroatien/tag3/IMG_0203.jpg"

func TestCheckingAPatternAllocatesNothing(t *testing.T) {
	b := bookOf("*.tmp", "*.partial*", "Thumbs*.db")
	anna := &users.User{Username: "anna"}
	if n := testing.AllocsPerRun(100, func() {
		if b.Effect(anna, deepPath) != ccfs.Open {
			t.Fatal("no pattern fits this path")
		}
	}); n != 0 {
		t.Errorf("%v allocations for one path", n)
	}
}

func BenchmarkEffectOfTwentyPatternsOnADeepPath(b *testing.B) {
	var patterns []string
	for i := range 20 {
		patterns = append(patterns, fmt.Sprintf("*.nix%d", i))
	}
	book := bookOf(patterns...)
	anna := &users.User{Username: "anna"}
	for b.Loop() {
		book.Effect(anna, deepPath)
	}
}

func TestPatternsMatchLikeTheirRegularExpression(t *testing.T) {
	patterns := []string{
		"*", "**", "*.tmp", "*.TMP", "IMG*", "img*.JPG", "*temp*", "a*b*c", "a**c", "*a*a*", "*.partial*", "x*y",
		"/mnt/user/*", "*/tag3/*", "*(1)*", "a.b*", "[x]*", "*k*", "*s*", "größe*", "*ß", "Kel*",
	}
	texts := []string{
		"", "a", "ac", "abc", "aXbYc", "acb", "aa", "aba", "x", "xy", "yx", "foo.tmp", "foo.TMP", "foo.tmp.bak", ".tmp", "tmp",
		"IMG_0203.jpg", "img_0203.JPG", "datei.partial", "datei.PARTIAL.1", "a.b", "axb", "[x]", "kopie (1).txt",
		deepPath, "/mnt/user", "/mnt/user/", "tag3",
		"K", "K", "Kelvin", "s", "ſ", "ſecret", "größe.txt", "GRÖSSE.txt", "straße", "zeile\numbruch.tmp", "zeile\nk", "\xff.tmp", "\x01.tmp",
	}
	for _, p := range patterns {
		c := compile(Rule{Path: p})
		for _, s := range texts {
			if got, want := c.matches(s), c.glob.MatchString(s); got != want {
				t.Errorf("pattern %q on %q: %v, the regular expression says %v", p, s, got, want)
			}
		}
	}
}
