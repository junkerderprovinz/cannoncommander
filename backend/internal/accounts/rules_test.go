package accounts_test

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

type rule = map[string]string

func (w *world) setRules(rules ...rule) {
	w.t.Helper()
	w.must(http.StatusOK, w.as(w.admin.Username, "PUT", "/admin/rules", map[string]any{"items": rules}))
}

func (w *world) verdict(user, path string) accounts.Verdict {
	w.t.Helper()
	return decode[accounts.Verdict](w.t, w.must(http.StatusOK, w.as(w.admin.Username, "POST", "/admin/rules/test", map[string]string{"user": user, "path": path})))
}

func TestGroupsKeepNoteAndMembers(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie", "note": "Fotos", "members": []string{"anna", "jdp", "anna"}}))
	w.must(http.StatusConflict, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie"}))
	w.must(http.StatusBadRequest, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "Familie!"}))
	w.must(http.StatusBadRequest, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "gaeste", "members": []string{"niemand"}}))

	list := decode[struct{ Items []accounts.Group }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/groups", nil))).Items
	if len(list) != 1 || list[0].Note != "Fotos" || !slices.Equal(list[0].Members, []string{"anna", "jdp"}) {
		t.Fatalf("groups = %+v", list)
	}
	if me := decode[accounts.Me](t, w.must(http.StatusOK, w.as("anna", "GET", "/me", nil))); !slices.Equal(me.Groups, []string{"familie"}) {
		t.Fatalf("anna is in %v", me.Groups)
	}
}

func TestRenamingAndDeletingAGroupMovesItsRules(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie", "note": "Fotos", "members": []string{"anna"}}))
	w.setRules(rule{"who": "@familie", "path": "/mnt/user/backups", "rule": "hide"}, rule{"who": "*", "path": "*.tmp", "rule": "hide"})

	got := decode[accounts.Group](t, w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/groups/familie", map[string]any{"name": "sippe"})))
	if got.Name != "sippe" || got.Note != "Fotos" || !slices.Equal(got.Members, []string{"anna"}) || state.GroupExists("familie") {
		t.Fatalf("renamed = %+v", got)
	}
	rules := decode[struct{ Items []accounts.Rule }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/rules", nil))).Items
	if rules[0].Who != "@sippe" {
		t.Fatalf("rules = %+v", rules)
	}
	w.must(http.StatusNotFound, w.as(w.admin.Username, "PATCH", "/admin/groups/familie", map[string]any{"note": "x"}))

	w.must(http.StatusNoContent, w.as(w.admin.Username, "DELETE", "/admin/groups/sippe", nil))
	rules = decode[struct{ Items []accounts.Rule }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/rules", nil))).Items
	if len(rules) != 1 || rules[0].Who != "*" {
		t.Fatalf("rules after delete = %+v", rules)
	}
	if _, err := state.GetUserByUsername("anna"); err != nil {
		t.Fatal("the members stay users")
	}
}

func TestRulesAreChecked(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	for _, bad := range []rule{
		{"who": "anna", "path": "", "rule": "hide"},
		{"who": "anna", "path": "/mnt/user/nirgends", "rule": "hide"},
		{"who": "anna", "path": "mnt/user", "rule": "hide"},
		{"who": "anna", "path": "/mnt/user/a.zip::/x", "rule": "hide"},
		{"who": "niemand", "path": "/mnt/user", "rule": "hide"},
		{"who": "@niemand", "path": "/mnt/user", "rule": "hide"},
		{"who": "", "path": "/mnt/user", "rule": "hide"},
		{"who": "anna", "path": "/mnt/user", "rule": "verstecken"},
	} {
		rec := w.as(w.admin.Username, "PUT", "/admin/rules", map[string]any{"items": []rule{{"who": "*", "path": "*.tmp", "rule": "hide"}, bad}})
		if rec.Code != http.StatusBadRequest || !strings.HasPrefix(detail(t, rec), "rule 2:") {
			t.Errorf("%v: %d %s", bad, rec.Code, rec.Body)
		}
	}
	got := decode[struct{ Items []accounts.Rule }](t, w.must(http.StatusOK, w.as(w.admin.Username, "PUT", "/admin/rules", map[string]any{"items": []rule{{"who": "anna", "path": " /mnt/user/backups/ ", "rule": "read"}}}))).Items
	if got[0].Path != "/mnt/user/backups" {
		t.Fatalf("the path is not cleaned: %+v", got)
	}
	w.must(http.StatusForbidden, w.as("anna", "PUT", "/admin/rules", map[string]any{"items": []rule{}}))
}

func TestTheFirstMatchingRuleDecides(t *testing.T) {
	w := newWorld(t)
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie"}))
	anna := w.addUser("anna", "user", "/mnt/user", "familie")
	bert := w.addUser("bert", "user", "/mnt/user")
	w.setRules(
		rule{"who": "anna", "path": "/mnt/user/backups/anna", "rule": "allow"},
		rule{"who": "@familie", "path": "/mnt/user/backups", "rule": "hide"},
		rule{"who": "*", "path": "*.PARTIAL", "rule": "hide"},
		rule{"who": "*", "path": "/mnt/user/filme", "rule": "read"},
	)
	cases := []struct {
		user *string
		path string
		perm ccfs.Perm
		want error
	}{
		{&anna.Username, "/mnt/user/backups/anna/flash.zip", ccfs.Modify, nil},
		{&anna.Username, "/mnt/user/backups", ccfs.View, ccfs.ErrForbidden},
		{&anna.Username, "/mnt/user/backups/bert", ccfs.View, ccfs.ErrForbidden},
		{&bert.Username, "/mnt/user/backups", ccfs.Modify, nil},
		{&bert.Username, "/mnt/user/downloads/film.mkv.partial", ccfs.View, ccfs.ErrForbidden},
		{&bert.Username, "/mnt/user/filme/a.mkv", ccfs.View, nil},
		{&bert.Username, "/mnt/user/filme/a.mkv", ccfs.Delete, ccfs.ErrReadOnly},
	}
	for _, c := range cases {
		u, _ := state.GetUserByUsername(*c.user)
		if _, err := w.fs.Check(&u, c.path, c.perm); !errors.Is(err, c.want) {
			t.Errorf("%s %s: %v, want %v", *c.user, c.path, err, c.want)
		}
	}
	dir, err := w.fs.Check(&bert, "/mnt/user/downloads", ccfs.View)
	if err != nil {
		t.Fatal(err)
	}
	if w.fs.Visible(&bert, dir, "film.mkv.partial") {
		t.Fatal("a hidden file shows up in the listing")
	}
}

func TestRulesSurviveARestart(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user")
	w.setRules(rule{"who": "anna", "path": "/mnt/user/backups", "rule": "hide"})
	v1.Forget(w.deps, "accounts")
	rules, err := accounts.Rules(w.deps.DB)
	if err != nil {
		t.Fatal(err)
	}
	anna, _ := state.GetUserByUsername("anna")
	if rules.Effect(&anna, "/mnt/user/backups/x") != ccfs.Hidden {
		t.Fatal("the rules were not loaded again")
	}
}

func TestTheCheckerExplainsWhatAUserGets(t *testing.T) {
	w := newWorld(t)
	w.addUser("anna", "user", "/mnt/user/fotos")
	w.addUser("gast", "readonly", "/mnt/user")
	w.addUser("bert", "user", "/mnt/user")
	w.setRules(rule{"who": "bert", "path": "/mnt/user/backups", "rule": "hide"}, rule{"who": "bert", "path": "/mnt/user/filme", "rule": "read"})
	one := func(i int) *int { return &i }
	cases := []struct {
		user, path string
		want       accounts.Verdict
	}{
		{"anna", "/mnt/user/filme", accounts.Verdict{Result: "hidden", Reason: "scope", Scope: "/mnt/user/fotos"}},
		{"anna", "/mnt/user/fotos/2026", accounts.Verdict{Result: "visible", Reason: "none"}},
		{"anna", "/mnt/user/nirgends", accounts.Verdict{Result: "missing", Reason: "none"}},
		{"gast", "/mnt/user/filme", accounts.Verdict{Result: "read", Reason: "role"}},
		{"bert", "/mnt/user/backups/anna", accounts.Verdict{Result: "hidden", Reason: "rule", Rule: one(0)}},
		{"bert", "/mnt/user/filme", accounts.Verdict{Result: "read", Reason: "rule", Rule: one(1)}},
		{"jdp", "/boot", accounts.Verdict{Result: "read", Reason: "source"}},
		{"bert", "/mnt/disk1", accounts.Verdict{Result: "hidden", Reason: "scope", Scope: "/mnt/user"}},
	}
	for _, c := range cases {
		got := w.verdict(c.user, c.path)
		if got.Result != c.want.Result || got.Reason != c.want.Reason || got.Scope != c.want.Scope || (got.Rule == nil) != (c.want.Rule == nil) || (got.Rule != nil && *got.Rule != *c.want.Rule) {
			t.Errorf("%s %s = %+v, want %+v", c.user, c.path, got, c.want)
		}
	}
	w.must(http.StatusNotFound, w.as(w.admin.Username, "POST", "/admin/rules/test", map[string]string{"user": "niemand", "path": "/mnt/user"}))
	w.must(http.StatusBadRequest, w.as(w.admin.Username, "POST", "/admin/rules/test", map[string]string{"user": "anna", "path": "mnt"}))
}
