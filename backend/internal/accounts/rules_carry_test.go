package accounts

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func carryBook(rules ...Rule) *ruleBook {
	b := &ruleBook{groupsOf: func(name string) []string {
		if name == "anna" {
			return []string{"familie"}
		}
		return nil
	}}
	for _, r := range rules {
		b.rules = append(b.rules, compile(r))
	}
	return b
}

func hide(who, p string) Rule  { return Rule{Who: who, Path: p, Rule: ruleHide} }
func read(who, p string) Rule  { return Rule{Who: who, Path: p, Rule: ruleReadOnly} }
func allow(who, p string) Rule { return Rule{Who: who, Path: p, Rule: ruleAllow} }

func TestWhatARuleSaysAboutAnEntryCarriedWithItsFolder(t *testing.T) {
	cases := []struct {
		name  string
		rules []Rule
		path  string
		want  ccfs.Effect
	}{
		{"a name pattern", []Rule{hide("*", "Thumbs*")}, "/mnt/user/a/Thumbs.db", ccfs.Open},
		{"an extension", []Rule{hide("*", "*.tmp")}, "/mnt/user/a/x.tmp", ccfs.Open},
		{"inside a folder a name pattern hides", []Rule{hide("*", "cache*")}, "/mnt/user/a/cache1/x", ccfs.Open},
		{"a name pattern for a group", []Rule{hide("@familie", "*.tmp")}, "/mnt/user/a/x.tmp", ccfs.Open},
		{"a name pattern for a user", []Rule{hide("anna", "*.tmp")}, "/mnt/user/a/x.tmp", ccfs.Open},
		{"a path", []Rule{hide("*", "/mnt/user/anna/private")}, "/mnt/user/anna/private", ccfs.Hidden},
		{"below a path", []Rule{hide("*", "/mnt/user/anna/private")}, "/mnt/user/anna/private/x", ccfs.Hidden},
		{"a path for a group", []Rule{hide("@familie", "/mnt/user/anna/private")}, "/mnt/user/anna/private", ccfs.Hidden},
		{"a path for another group", []Rule{hide("@gaeste", "/mnt/user/anna/private")}, "/mnt/user/anna/private", ccfs.Open},
		{"a name pattern below a given folder", []Rule{hide("*", "/mnt/user/anna/*.tmp")}, "/mnt/user/anna/sub/x.tmp", ccfs.Hidden},
		{"beside the given folder", []Rule{hide("*", "/mnt/user/anna/*.tmp")}, "/mnt/user/bert/x.tmp", ccfs.Open},
		{"a star in the middle of a path", []Rule{hide("*", "appdata/*/cache")}, "/mnt/user/appdata/plex/cache", ccfs.Hidden},
		{"below a star in the middle of a path", []Rule{hide("*", "appdata/*/cache")}, "/mnt/user/appdata/plex/cache/x", ccfs.Hidden},
		{"a pattern that fits every name", []Rule{hide("*", "*")}, "/mnt/user/a/x", ccfs.Hidden},
		{"only stars", []Rule{hide("*", "**")}, "/mnt/user/a/x", ccfs.Hidden},
		{"a read rule on a path", []Rule{read("*", "/mnt/user/a")}, "/mnt/user/a/x", ccfs.ReadOnly},
		{"a read rule on a name", []Rule{read("*", "*.tmp")}, "/mnt/user/a/x.tmp", ccfs.ReadOnly},
		{"a path rule behind a name pattern", []Rule{hide("*", "*.tmp"), hide("*", "/mnt/user/anna/private")}, "/mnt/user/anna/private/x.tmp", ccfs.Hidden},
		{"a read rule behind a name pattern", []Rule{hide("*", "*.tmp"), read("*", "/mnt/user/a")}, "/mnt/user/a/x.tmp", ccfs.ReadOnly},
		{"an allow between a name pattern and a path", []Rule{hide("*", "*.tmp"), allow("*", "/mnt/user/a"), hide("*", "/mnt/user")}, "/mnt/user/a/x.tmp", ccfs.Open},
		{"an allow above a path", []Rule{allow("*", "/mnt/user/a/pub"), hide("*", "/mnt/user/a")}, "/mnt/user/a/pub/x", ccfs.Open},
	}
	anna := &users.User{Username: "anna"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := carryBook(c.rules...).CarryEffect(anna, c.path); got != c.want {
				t.Errorf("%v, want %v", got, c.want)
			}
		})
	}
}

func TestWhereCarryingAFolderTakesALookInside(t *testing.T) {
	cases := []struct {
		name  string
		rules []Rule
		path  string
		want  bool
	}{
		{"no rules", nil, "/mnt/user/a", false},
		{"name patterns alone", []Rule{hide("*", "Thumbs*"), hide("*", "*.tmp"), hide("@familie", ".*")}, "/mnt/user/a", false},
		{"a path below", []Rule{hide("*", "/mnt/user/a/b")}, "/mnt/user/a", true},
		{"the path itself", []Rule{hide("*", "/mnt/user/a")}, "/mnt/user/a", true},
		{"a path above", []Rule{hide("*", "/mnt/user")}, "/mnt/user/a", true},
		{"a path beside", []Rule{hide("*", "/mnt/user/b")}, "/mnt/user/a", false},
		{"a path sharing the start of the name", []Rule{hide("*", "/mnt/user/ab")}, "/mnt/user/a", false},
		{"a path of another group", []Rule{hide("@gaeste", "/mnt/user/a/b")}, "/mnt/user/a", false},
		{"a path of the own group", []Rule{hide("@familie", "/mnt/user/a/b")}, "/mnt/user/a", true},
		{"a read pattern", []Rule{read("*", "*.tmp")}, "/mnt/user/a", true},
		{"a pattern below a given folder here", []Rule{hide("*", "/mnt/user/a/*.tmp")}, "/mnt/user/a", true},
		{"a pattern below a given folder further down", []Rule{hide("*", "/mnt/user/a/b/*.tmp")}, "/mnt/user/a", true},
		{"a pattern below a given folder above", []Rule{hide("*", "/mnt/user/*.tmp")}, "/mnt/user/a", true},
		{"a pattern below a given folder beside", []Rule{hide("*", "/mnt/user/b/*.tmp")}, "/mnt/user/a", false},
		{"a star in the middle of a path", []Rule{hide("*", "appdata/*/cache")}, "/mnt/user/a", true},
		{"a pattern that fits every name", []Rule{hide("*", "*")}, "/mnt/user/a", true},
		{"an allow", []Rule{allow("*", "/mnt/user/a/b")}, "/mnt/user/a", false},
	}
	anna := &users.User{Username: "anna"}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := carryBook(c.rules...).CarryBelow(anna, c.path); got != c.want {
				t.Errorf("%v, want %v", got, c.want)
			}
		})
	}
}

func TestAPatternWithAFolderPartCoversWhatLiesBelowItsMatch(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          ccfs.Effect
	}{
		{"/mnt/user/appdata/*/cache", "/mnt/user/appdata/plex/cache", ccfs.Hidden},
		{"/mnt/user/appdata/*/cache", "/mnt/user/appdata/plex/cache/a/b", ccfs.Hidden},
		{"/mnt/user/appdata/*/cache", "/mnt/user/appdata/plex/cached", ccfs.Open},
		{"/mnt/user/appdata/*/cache", "/mnt/user/appdata/plex", ccfs.Open},
		{"appdata/*/cache", "/mnt/user/appdata/plex/cache/a", ccfs.Hidden},
		{"appdata/*/cache", "/mnt/user/myappdata/plex/cache", ccfs.Open},
		{"APPDATA/*/CACHE", "/mnt/user/appdata/plex/cache", ccfs.Hidden},
		{"/mnt/user/anna/*.tmp", "/mnt/user/anna/x.tmp/inside", ccfs.Hidden},
	}
	anna := &users.User{Username: "anna"}
	for _, c := range cases {
		if got := carryBook(hide("*", c.pattern)).Effect(anna, c.path); got != c.want {
			t.Errorf("%s on %s: %v, want %v", c.pattern, c.path, got, c.want)
		}
	}
}
