package accounts_test

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

func TestAdminsCreateUsersWithRoleScopeAndGroups(t *testing.T) {
	w := newWorld(t).open()
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie", "note": "Fotos und Filme"}))
	anna := w.addUser("anna", "user", "/mnt/user/fotos", "familie")

	got := decode[accounts.Account](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/users/anna", nil)))
	if got.Name != "anna" || got.Full != "Anna" || got.Role != "user" || got.Scope != "/mnt/user/fotos" || !slices.Equal(got.Groups, []string{"familie"}) || got.Last != nil || got.PwChanged == nil {
		t.Fatalf("anna = %+v", got)
	}
	if byID := decode[accounts.Account](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/users/"+got.ID, nil))); byID.Name != "anna" {
		t.Fatalf("by id = %+v", byID)
	}
	if _, err := w.signIn("anna", strong); err != nil {
		t.Fatalf("the password did not take: %v", err)
	}
	if utils.CheckPwd(strong, anna.Password) == nil {
		t.Fatal("the users table keeps the password the version before would let in with")
	}
	if _, err := w.fs.Check(&anna, "/mnt/user/fotos/2026", ccfs.Create); err != nil {
		t.Fatalf("a user writes in the scope: %v", err)
	}
	if _, err := w.fs.Check(&anna, "/mnt/user/filme", ccfs.View); err == nil {
		t.Fatal("a user sees beyond the scope")
	}

	gast := w.addUser("gast", "readonly", "/mnt/user/filme")
	if _, err := w.fs.Check(&gast, "/mnt/user/filme", ccfs.View|ccfs.Download); err != nil {
		t.Fatalf("read only reads: %v", err)
	}
	if _, err := w.fs.Check(&gast, "/mnt/user/filme", ccfs.Delete); err == nil {
		t.Fatal("read only deletes")
	}

	list := decode[struct{ Items []accounts.Account }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/users", nil))).Items
	if len(list) != 3 {
		t.Fatalf("list = %+v", list)
	}
	w.must(http.StatusForbidden, w.as("anna", "GET", "/admin/users", nil))
}

func TestNewUsersAreChecked(t *testing.T) {
	w := newWorld(t).open()
	w.addUser("anna", "user", "/mnt/user")
	cases := []struct {
		body map[string]any
		code int
	}{
		{map[string]any{"name": "", "scope": "/mnt/user", "password": strong}, http.StatusBadRequest},
		{map[string]any{"name": "Anna2", "scope": "/mnt/user", "password": strong}, http.StatusBadRequest},
		{map[string]any{"name": "a", "scope": "/mnt/user", "password": strong}, http.StatusBadRequest},
		{map[string]any{"name": "anna", "scope": "/mnt/user", "password": strong}, http.StatusConflict},
		{map[string]any{"name": "bert", "scope": "/mnt/user"}, http.StatusBadRequest},
		{map[string]any{"name": "bert", "scope": "/mnt/user", "password": ""}, http.StatusBadRequest},
		{map[string]any{"name": "bert", "password": strong}, http.StatusBadRequest},
		{map[string]any{"name": "bert", "scope": "/mnt/user/nirgends", "password": strong}, http.StatusBadRequest},
		{map[string]any{"name": "bert", "scope": "/mnt/disk1", "password": strong}, http.StatusBadRequest},
		{map[string]any{"name": "bert", "scope": "/mnt/user", "password": strong, "role": "chef"}, http.StatusBadRequest},
		{map[string]any{"name": "bert", "scope": "/mnt/user", "password": strong, "groups": []string{"gibtsnicht"}}, http.StatusBadRequest},
	}
	for _, c := range cases {
		if rec := w.as(w.admin.Username, "POST", "/admin/users", c.body); rec.Code != c.code {
			t.Errorf("%v: %d, want %d: %s", c.body, rec.Code, c.code, rec.Body)
		}
	}
	if _, err := state.GetUserByUsername("bert"); err == nil {
		t.Fatal("a rejected user was created")
	}
}

func TestAFolderThatOnlyLeadsToSourcesStandsForAllOfThem(t *testing.T) {
	w := newWorld(t).open()
	boss := w.addUser("boss", "admin", "/mnt")
	if got := decode[accounts.Account](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/users/boss", nil))); got.Scope != "/mnt" {
		t.Fatalf("scope = %q", got.Scope)
	}
	if _, err := w.fs.Check(&boss, "/mnt/disk1", ccfs.View); err != nil {
		t.Fatalf("an admin sees the disks: %v", err)
	}
	anna := w.addUser("anna", "user", "/mnt")
	if _, err := w.fs.Check(&anna, "/mnt/disk1", ccfs.View); err == nil {
		t.Fatal("a user got the single disks")
	}
	if len(anna.BackendScopes) != 1 {
		t.Fatalf("anna got %+v", anna.BackendScopes)
	}
}

func TestChangingAUser(t *testing.T) {
	w := newWorld(t).open()
	anna := w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.as(w.admin.Username, "PUT", "/admin/rules", map[string]any{"items": []map[string]string{{"who": "anna", "path": "/mnt/user/backups", "rule": "hide"}}}))

	got := decode[accounts.Account](t, w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anna", map[string]any{"name": "anni", "full": "Anna Huber", "role": "readonly", "scope": "/mnt/user/fotos"})))
	if got.Name != "anni" || got.Full != "Anna Huber" || got.Role != "readonly" || got.Scope != "/mnt/user/fotos" {
		t.Fatalf("changed = %+v", got)
	}
	rules := decode[struct{ Items []accounts.Rule }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/rules", nil))).Items
	if len(rules) != 1 || rules[0].Who != "anni" {
		t.Fatalf("the rule did not follow the rename: %+v", rules)
	}

	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anni", map[string]any{"role": "user"}))
	anni, _ := state.GetUserByUsername("anni")
	if _, err := w.fs.Check(&anni, "/mnt/user/fotos", ccfs.Modify); err != nil {
		t.Fatalf("a new role alone must change the rights: %v", err)
	}

	before := w.session("anni")
	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anni", map[string]any{"password": "noch-ein-langes-passwort"}))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, header{"Cookie", before}))
	if _, err := w.signIn("anni", "noch-ein-langes-passwort"); err != nil {
		t.Fatalf("the new password does not sign in: %v", err)
	}
	if got := w.account(anna); got.Name != "anni" || got.Role != "user" {
		t.Fatalf("the sign-in did not follow: %+v", got)
	}
	w.must(http.StatusBadRequest, w.as(w.admin.Username, "PATCH", "/admin/users/anni", map[string]any{"password": ""}))
	w.must(http.StatusNotFound, w.as(w.admin.Username, "PATCH", "/admin/users/anna", map[string]any{"full": "x"}))
}

func TestAdminsCannotLockThemselvesOut(t *testing.T) {
	w := newWorld(t).open()
	me := w.admin.Username
	w.must(http.StatusConflict, w.as(me, "PATCH", "/admin/users/"+me, map[string]any{"role": "user"}))
	w.must(http.StatusBadRequest, w.as(me, "PATCH", "/admin/users/"+me, map[string]any{"name": "chef"}))
	w.must(http.StatusBadRequest, w.as(me, "PATCH", "/admin/users/"+me, map[string]any{"password": strong}))
	w.must(http.StatusForbidden, w.as(me, "DELETE", "/admin/users/"+me, nil))

	w.addUser("zweite", "admin", "/mnt/user")
	w.must(http.StatusOK, w.as("zweite", "PATCH", "/admin/users/"+me, map[string]any{"role": "user"}))
	// A lower role ends the sessions of the account.
	w.must(http.StatusUnauthorized, w.as(me, "GET", "/me", nil))
	delete(w.cookies, me)
	w.must(http.StatusForbidden, w.as(me, "PATCH", "/admin/users/zweite", map[string]any{"role": "user"}))
}

func TestDeletingAUserTakesItsTracesAlong(t *testing.T) {
	w := newWorld(t).open()
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie"}))
	anna := w.addUser("anna", "user", "/mnt/user", "familie")
	tok := decode[struct{ Value string }](t, w.must(http.StatusOK, w.as("anna", "POST", "/api/signin/v1/me/tokens", map[string]any{"name": "skript", "scopes": []string{"all"}})))
	w.must(http.StatusOK, w.req("GET", "/me", nil, header{"Authorization", "Bearer " + tok.Value}))
	w.must(http.StatusOK, w.as(w.admin.Username, "PUT", "/admin/rules", map[string]any{"items": []map[string]string{
		{"who": "anna", "path": "/mnt/user/backups", "rule": "hide"},
		{"who": "@familie", "path": "/mnt/user/filme", "rule": "read"},
	}}))
	anna.Permissions.Share = true
	if err := state.UpdateUser(&anna, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	w.must(http.StatusCreated, w.as("anna", "POST", "/shares", map[string]any{"paths": []string{"/mnt/user/fotos"}}))
	if links := decode[[]any](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/shares", nil))); len(links) != 1 {
		t.Fatalf("share links before = %v", links)
	}
	id := strconv.FormatUint(anna.ID, 10)
	if _, err := w.deps.DB.Exec(`INSERT INTO user_prefs (user_id, data, updated_at) VALUES (?, '{}', 0)`, id); err != nil {
		t.Fatal(err)
	}

	w.must(http.StatusNoContent, w.as(w.admin.Username, "DELETE", "/admin/users/anna", nil))

	if _, err := state.GetUserByUsername("anna"); err == nil {
		t.Fatal("anna is still there")
	}
	if links := decode[[]any](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/shares", nil))); len(links) != 0 {
		t.Fatalf("share links left: %+v", links)
	}
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, header{"Authorization", "Bearer " + tok.Value}))
	rules := decode[struct{ Items []accounts.Rule }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/rules", nil))).Items
	if len(rules) != 1 || rules[0].Who != "@familie" {
		t.Fatalf("rules = %+v", rules)
	}
	if slices.Contains(state.GetGroupMembers()["familie"], "anna") {
		t.Fatal("anna is still in the group")
	}
	for _, table := range []string{"user_prefs", "sessions", "account_profiles", "token_use", "synced_members"} {
		var n int
		where := "user_id = ?"
		arg := any(id)
		if table == "synced_members" {
			where, arg = "username = ?", "anna"
		}
		if err := w.deps.DB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE `+where, arg).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s keeps %d rows of anna, %v", table, n, err)
		}
	}
	w.must(http.StatusNotFound, w.as(w.admin.Username, "DELETE", "/admin/users/anna", nil))
}

func TestTheRoleDecidesWhoMayShare(t *testing.T) {
	w := newWorld(t).open()
	w.addUser("anna", "user", "/mnt/user/fotos")
	w.addUser("gast", "readonly", "/mnt/user/fotos")
	w.addUser("boss", "admin", "/mnt/user/fotos")
	link := map[string]any{"paths": []string{"/mnt/user/fotos"}}
	w.must(http.StatusCreated, w.as("anna", "POST", "/shares", link))
	w.must(http.StatusCreated, w.as("boss", "POST", "/shares", link))
	w.must(http.StatusForbidden, w.as("gast", "POST", "/shares", link))

	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/gast", map[string]any{"role": "user"}))
	w.must(http.StatusCreated, w.as("gast", "POST", "/shares", link))
	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anna", map[string]any{"role": "readonly"}))
	delete(w.cookies, "anna")
	w.must(http.StatusForbidden, w.as("anna", "POST", "/shares", link))
}

// traces leaves an undo step and a trash entry under a login name, as work
// with files does, and returns the entry with the file it holds.
func (w *world) traces(owner string) (*undo.Log, *trash.Store, trash.Item, string) {
	w.t.Helper()
	log, bin := undo.For(w.deps), trash.For(w.deps)
	w.t.Cleanup(func() {
		bin.Close()
		v1.Forget(w.deps, "trash")
		v1.Forget(w.deps, "undo")
	})
	if _, err := log.Record(owner, undo.Step{Kind: "test", Name: "alt.txt", Count: 1}, nil); err != nil {
		w.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.share, "fotos", "alt.txt"), []byte("alt"), 0o644); err != nil {
		w.t.Fatal(err)
	}
	loc, err := w.fs.Resolve("/mnt/user/fotos/alt.txt")
	if err != nil {
		w.t.Fatal(err)
	}
	it, err := bin.Put(w.t.Context(), owner, loc)
	if err != nil {
		w.t.Fatal(err)
	}
	return log, bin, it, filepath.Join(w.share, "fotos", trash.DirName, it.ID, "alt.txt")
}

func TestANewUserOfAnOldNameInheritsNoUndoStepsOrTrash(t *testing.T) {
	w := newWorld(t).open()
	w.addUser("anna", "user", "/mnt/user")
	log, bin, it, file := w.traces("anna")

	w.must(http.StatusNoContent, w.as(w.admin.Username, "DELETE", "/admin/users/anna", nil))
	w.addUser("anna", "user", "/mnt/user")

	if steps, err := log.List("anna"); err != nil || len(steps) != 0 {
		t.Errorf("undo steps of the old anna: %+v, %v", steps, err)
	}
	if items, err := bin.List("anna"); err != nil || len(items) != 0 {
		t.Errorf("trash of the old anna: %+v, %v", items, err)
	}

	if _, err := os.Stat(file); err != nil {
		t.Fatalf("the entry left the bin before its time: %v", err)
	}
	if _, err := w.deps.DB.Exec(`UPDATE trash_items SET deleted_at = 1 WHERE id = ?`, it.ID); err != nil {
		t.Fatal(err)
	}
	if err := bin.SetDays(30); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(file); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the file outlived its time in the bin: %v", err)
	}
	if _, err := bin.Get(it.ID); !errors.Is(err, trash.ErrNotFound) {
		t.Errorf("the entry outlived its time in the bin: %v", err)
	}
}

func TestRenamingAUserTakesUndoStepsTrashAndJobsAlong(t *testing.T) {
	w := newWorld(t).open()
	w.addUser("anna", "user", "/mnt/user")
	log, bin, it, _ := w.traces("anna")
	m := jobs.For(w.deps)
	t.Cleanup(func() {
		m.Close()
		v1.Forget(w.deps, "jobs")
	})
	job, err := m.Submit("anna", trash.KindEmpty, &jobs.Plan{Params: struct{}{}, Items: []jobs.Item{{Path: "/mnt/user/fotos/x", Name: "x", Ref: "gibtsnicht"}}}, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anna", map[string]any{"name": "anni"}))

	if steps, err := log.List("anni"); err != nil || len(steps) != 1 {
		t.Errorf("undo steps under the new name: %+v, %v", steps, err)
	}
	if steps, err := log.List("anna"); err != nil || len(steps) != 0 {
		t.Errorf("undo steps under the old name: %+v, %v", steps, err)
	}
	if items, err := bin.List("anni"); err != nil || len(items) != 1 || items[0].ID != it.ID {
		t.Errorf("trash under the new name: %+v, %v", items, err)
	}
	if items, err := bin.List("anna"); err != nil || len(items) != 0 {
		t.Errorf("trash under the old name: %+v, %v", items, err)
	}
	if list := m.List("anni"); len(list) != 1 || list[0].ID != job.ID {
		t.Errorf("jobs under the new name: %+v", list)
	}
	if list := m.List("anna"); len(list) != 0 {
		t.Errorf("jobs under the old name: %+v", list)
	}
	var owner string
	if err := w.deps.DB.QueryRow(`SELECT owner FROM jobs WHERE id = ?`, job.ID).Scan(&owner); err != nil || owner != "anni" {
		t.Errorf("the stored job belongs to %q, %v", owner, err)
	}
}

func TestARoleSetInTheAppReachesTheRowTheOlderVersionReads(t *testing.T) {
	w := newWorld(t).open()
	anna := w.addUser("anna", "admin", "/mnt/user")
	permAdmin := func() (admin bool) {
		if err := w.deps.DB.QueryRow(`SELECT perm_admin FROM users WHERE user_id = ?`, strconv.FormatUint(anna.ID, 10)).Scan(&admin); err != nil {
			t.Fatal(err)
		}
		return admin
	}
	if !permAdmin() {
		t.Fatal("a new admin is no admin in the row")
	}
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"role": "user"}))
	if permAdmin() {
		t.Fatal("a demoted admin is still admin for the version before")
	}
}

func TestTheUsersTileTakesAPasswordOfAnyLength(t *testing.T) {
	w := newWorld(t).open()
	type answer struct{ Short bool }
	rec := w.must(http.StatusCreated, w.as("jdp", "POST", "/admin/users", map[string]any{"name": "kurt", "role": "user", "scope": "/mnt/user", "password": "ab"}))
	if !decode[answer](t, rec).Short {
		t.Fatal("the answer does not say the password is short")
	}
	if _, err := w.signIn("kurt", "ab"); err != nil {
		t.Fatalf("the short password does not sign in: %v", err)
	}
	rec = w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/kurt", map[string]any{"password": "x"}))
	if !decode[answer](t, rec).Short {
		t.Fatal("the change does not say the password is short")
	}
	rec = w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/kurt", map[string]any{"full": "Kurt"}))
	if decode[answer](t, rec).Short {
		t.Fatal("a change without a password calls it short")
	}
	if _, err := w.signIn("kurt", "x"); err != nil {
		t.Fatalf("the one-letter password does not sign in: %v", err)
	}
	w.must(http.StatusBadRequest, w.as("jdp", "POST", "/admin/users", map[string]any{"name": "leer", "role": "user", "scope": "/mnt/user", "password": ""}))
	w.must(http.StatusBadRequest, w.as("jdp", "PATCH", "/admin/users/kurt", map[string]any{"name": "kurti", "password": ""}))
	if _, err := state.GetUserByUsername("kurt"); err != nil {
		t.Fatalf("a change with an empty password landed halfway: %v", err)
	}
}

func TestALongPasswordIsNotCalledShort(t *testing.T) {
	w := newWorld(t).open()
	rec := w.must(http.StatusCreated, w.as("jdp", "POST", "/admin/users", map[string]any{"name": "kurt", "role": "user", "scope": "/mnt/user", "password": strong}))
	if decode[struct{ Short bool }](t, rec).Short {
		t.Fatal("a long password is called short")
	}
}

func TestTheUsersTileWorksWhileNobodyHasAPassword(t *testing.T) {
	w := newBareWorld(t)
	w.admin = w.oldUser("jdp", "", true, users.BackendScope{Path: w.share, Scope: "/"})
	w.oldUser("kurt", "", false, users.BackendScope{Path: w.share, Scope: "/"})
	w.open()
	w.must(http.StatusOK, w.req("GET", "/admin/users", nil))
	w.must(http.StatusOK, w.req("PATCH", "/admin/users/kurt", map[string]any{"name": "kurti", "role": "readonly", "blocked": true}))
	kurti, err := state.GetUserByUsername("kurti")
	if err != nil {
		t.Fatalf("the rename while open did not land: %v", err)
	}
	got := w.account(kurti)
	if got.Role != "readonly" || !got.Blocked {
		t.Fatalf("the change while open did not land: %+v", got)
	}
	w.must(http.StatusNoContent, w.req("DELETE", "/admin/users/kurti", nil))
	if _, err := state.GetUserByUsername("kurti"); err == nil {
		t.Fatal("the account removed while open is still there")
	}
}
