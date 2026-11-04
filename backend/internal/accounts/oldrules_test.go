package accounts

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// decodeOld reads a rule the way the old tables stored it.
func decodeOld(t *testing.T, data string) oldRule {
	t.Helper()
	var r oldRule
	if err := json.Unmarshal([]byte(data), &r); err != nil {
		t.Fatal(err)
	}
	return r
}

func mkdirs(t *testing.T, root string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func knows(list ...string) func(string) bool {
	return func(who string) bool { return slices.Contains(list, who) }
}

func TestTheRulesOfAnOldDatabaseCarryOver(t *testing.T) {
	// The two rules and the source setting of the database the old
	// interface used in its own tests: admin may not see one script, basic
	// may not see the source "access", which nobody sees by default.
	files, access := t.TempDir(), t.TempDir()
	mkdirs(t, files, "text-files")
	if err := os.WriteFile(filepath.Join(files, "text-files", "bash.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rows := []oldRow{
		{source: "/app/frontend/tests/playwright-files", path: "/text-files/bash.sh/", rule: decodeOld(t, `{"Deny":{"Users":{"admin":{}},"Groups":{}},"Allow":{"Users":{},"Groups":{}}}`)},
		{source: "/tests/playwright-files", path: "/", rule: decodeOld(t, `{"Deny":{"Users":{"basic":{}},"Groups":{}},"Allow":{"Users":{},"Groups":{}}}`)},
	}
	sources := []oldSource{
		{name: "playwright + files", key: "/app/frontend/tests/playwright-files", root: files, mount: "/mnt/user/files"},
		{name: "access", key: "/tests/playwright-files", root: access, mount: "/mnt/user/access", denyByDefault: true},
	}
	got := carryOver(rows, sources, knows("admin", "basic"), onDisk)
	want := []Rule{
		{Who: "basic", Path: "/mnt/user/access", Rule: ruleHide},
		{Who: everyone, Path: "/mnt/user/access", Rule: ruleHide},
		{Who: "admin", Path: "/mnt/user/files/text-files/bash.sh", Rule: ruleHide},
	}
	if !slices.Equal(got.rules, want) {
		t.Fatalf("rules = %v, want %v", got.rules, want)
	}
	if len(got.notes) != 0 {
		t.Fatalf("notes = %v", got.notes)
	}
}

// oldTree is a source with rules on several levels and every case that
// does not map one to one.
func oldTree(t *testing.T) ([]oldRow, []oldSource, func(string) bool) {
	root := t.TempDir()
	mkdirs(t, root, "docs/private", "docs/public", "Music/live", "other")
	rows := []oldRow{
		{source: root, path: "/docs/", rule: oldRule{DenyAll: true}},
		{source: root, path: "/docs/", rule: decodeOld(t, `{"Deny":{"Users":{"ghost":{}}},"Allow":{"Users":{"alice":{}}}}`)},
		{source: root, path: "/docs/private/", rule: decodeOld(t, `{"Deny":{"Groups":{"team":{}}},"Allow":{"Users":{"bob":{}}}}`)},
		{source: "Shares", path: "/Music", rule: decodeOld(t, `{"Deny":{"Users":{"carol":{}}}}`)},
		{source: root, path: "/music/", rule: decodeOld(t, `{"Allow":{"Users":{"dave":{}}}}`)},
		{source: root, path: "/gone/", rule: decodeOld(t, `{"Deny":{"Users":{"alice":{}}}}`)},
		{source: root, path: "/", rule: decodeOld(t, `{"Allow":{"Groups":{"admins":{}}}}`)},
		{source: "/mnt/removed", path: "/", rule: decodeOld(t, `{"Deny":{"Users":{"alice":{}}}}`)},
	}
	sources := []oldSource{{name: "Shares", key: root, root: root, mount: "/mnt/user"}}
	return rows, sources, knows("alice", "bob", "carol", "dave", "erin", "@team", "@admins")
}

func TestOldRulesKeepTheirOrderOfPrecedence(t *testing.T) {
	rows, sources, known := oldTree(t)
	got := carryOver(rows, sources, known, onDisk)
	want := []Rule{
		{Who: "@team", Path: "/mnt/user/docs/private", Rule: ruleHide},
		{Who: "bob", Path: "/mnt/user/docs/private", Rule: ruleAllow},
		{Who: "alice", Path: "/mnt/user/docs", Rule: ruleAllow},
		{Who: "alice", Path: "/mnt/user/gone", Rule: ruleHide},
		{Who: "carol", Path: "/mnt/user/Music", Rule: ruleHide},
		{Who: "@admins", Path: "/mnt/user", Rule: ruleAllow},
		{Who: everyone, Path: "/mnt/user/docs", Rule: ruleHide},
	}
	if !slices.Equal(got.rules, want) {
		t.Fatalf("rules =\n%v\nwant\n%v", got.rules, want)
	}
	for _, part := range []string{"/mnt/removed", "/gone in Shares does not exist", "ghost", "dave"} {
		if !slices.ContainsFunc(got.notes, func(n string) bool { return strings.Contains(n, part) }) {
			t.Errorf("no note about %s in %v", part, got.notes)
		}
	}
}

// oldVerdict is how the old interface decided whether user may see p.
func oldVerdict(rules map[string]oldRule, user string, groups []string, p string, denyByDefault bool) bool {
	var found []oldRule
	for cur := strings.ToLower(p); ; cur = path.Dir(cur) {
		if r, ok := rules[cur]; ok {
			found = append(found, r)
		}
		if cur == "/" {
			break
		}
	}
	member := func(set oldNames) bool {
		return slices.ContainsFunc(groups, func(g string) bool { _, ok := set[g]; return ok })
	}
	for _, r := range found {
		if _, ok := r.Deny.Users[user]; ok {
			return false
		}
		if member(r.Deny.Groups) {
			return false
		}
		if _, ok := r.Allow.Users[user]; ok {
			return true
		}
		if member(r.Allow.Groups) {
			return true
		}
	}
	for _, r := range found {
		if r.DenyAll {
			return false
		}
	}
	return !denyByDefault
}

func TestCarriedRulesDecideLikeTheOldOnes(t *testing.T) {
	rows, sources, known := oldTree(t)
	for _, denyByDefault := range []bool{false, true} {
		sources[0].denyByDefault = denyByDefault
		carried := carryOver(rows, sources, known, onDisk)
		book := &ruleBook{groupsOf: func(name string) []string {
			return map[string][]string{"bob": {"team"}, "erin": {"admins"}, "carol": {"team"}}[name]
		}}
		for _, r := range carried.rules {
			book.rules = append(book.rules, compile(r))
		}
		old := map[string]oldRule{}
		for _, r := range rows {
			if r.source == "/mnt/removed" {
				continue
			}
			k := oldKey(r.path)
			m := old[k]
			merge(&m, r.rule)
			old[k] = m
		}
		for _, name := range []string{"alice", "bob", "carol", "dave", "erin", "frank"} {
			u := &users.User{FrontendUser: users.FrontendUser{Username: name}}
			for _, p := range []string{"/", "/docs", "/docs/a.txt", "/docs/private", "/docs/private/x", "/docs/public", "/Music", "/Music/live", "/other", "/other/y"} {
				want := oldVerdict(old, name, book.groupsOf(name), p, denyByDefault)
				hidden := book.Effect(u, path.Join("/mnt/user", p)) == ccfs.Hidden
				if hidden == want {
					t.Errorf("denyByDefault=%v %s at %s: old says visible=%v, the carried rules hide=%v", denyByDefault, name, p, want, hidden)
				}
			}
		}
	}
}

func TestEveryCaseOfANameOnDiskGetsARule(t *testing.T) {
	root := t.TempDir()
	mkdirs(t, root, "Photos")
	if err := os.Mkdir(filepath.Join(root, "photos"), 0o755); err != nil {
		t.Skip("the file system ignores case")
	}
	got := onDisk(root, "/photos")
	slices.Sort(got)
	if !slices.Equal(got, []string{"/Photos", "/photos"}) {
		t.Fatalf("onDisk = %v", got)
	}
}
