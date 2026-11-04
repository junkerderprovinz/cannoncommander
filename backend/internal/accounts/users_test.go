package accounts_test

import (
	"net/http"
	"slices"
	"strconv"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

func TestAdminsCreateUsersWithRoleScopeAndGroups(t *testing.T) {
	w := newWorld(t)
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie", "note": "Fotos und Filme"}))
	anna := w.addUser("anna", "user", "/mnt/user/fotos", "familie")

	got := decode[accounts.Account](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/users/anna", nil)))
	if got.Name != "anna" || got.Full != "Anna" || got.Role != "user" || got.Scope != "/mnt/user/fotos" || !slices.Equal(got.Groups, []string{"familie"}) || got.Last != nil || got.PwChanged == nil {
		t.Fatalf("anna = %+v", got)
	}
	if byID := decode[accounts.Account](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/users/"+got.ID, nil))); byID.Name != "anna" {
		t.Fatalf("by id = %+v", byID)
	}
	if utils.CheckPwd(strong, anna.Password) != nil {
		t.Fatal("the password did not take")
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
	w := newWorld(t)
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
		{map[string]any{"name": "bert", "scope": "/mnt/user", "password": "kurz"}, http.StatusBadRequest},
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
	w := newWorld(t)
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
	w := newWorld(t)
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

	w.must(http.StatusOK, w.as(w.admin.Username, "PATCH", "/admin/users/anni", map[string]any{"password": "noch-ein-langes-passwort"}))
	if last := w.login.endAll[len(w.login.endAll)-1]; last.user != anna.ID || last.keep != "" {
		t.Fatalf("a new password must log the user out everywhere: %+v", last)
	}
	w.must(http.StatusBadRequest, w.as(w.admin.Username, "PATCH", "/admin/users/anni", map[string]any{"password": "kurz"}))
	w.must(http.StatusNotFound, w.as(w.admin.Username, "PATCH", "/admin/users/anna", map[string]any{"full": "x"}))
}

func TestAdminsCannotLockThemselvesOut(t *testing.T) {
	w := newWorld(t)
	me := w.admin.Username
	w.must(http.StatusConflict, w.as(me, "PATCH", "/admin/users/"+me, map[string]any{"role": "user"}))
	w.must(http.StatusBadRequest, w.as(me, "PATCH", "/admin/users/"+me, map[string]any{"name": "chef"}))
	w.must(http.StatusBadRequest, w.as(me, "PATCH", "/admin/users/"+me, map[string]any{"password": strong}))
	w.must(http.StatusForbidden, w.as(me, "DELETE", "/admin/users/"+me, nil))

	w.addUser("zweite", "admin", "/mnt/user")
	w.must(http.StatusOK, w.as("zweite", "PATCH", "/admin/users/"+me, map[string]any{"role": "user"}))
	w.must(http.StatusForbidden, w.as(me, "PATCH", "/admin/users/zweite", map[string]any{"role": "user"}))
}

func TestDeletingAUserTakesItsTracesAlong(t *testing.T) {
	w := newWorld(t)
	w.must(http.StatusCreated, w.as(w.admin.Username, "POST", "/admin/groups", map[string]any{"name": "familie"}))
	anna := w.addUser("anna", "user", "/mnt/user", "familie")
	w.as("anna", "GET", "/me", nil)
	tok := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"name": "skript", "rights": "rw"})))
	w.req("GET", "/me", nil, "X-Api", tok.Secret)
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
	if _, _, ok := state.HashedTokenOwner(tok.Secret); ok {
		t.Fatal("the token of anna still logs in")
	}
	rules := decode[struct{ Items []accounts.Rule }](t, w.must(http.StatusOK, w.as(w.admin.Username, "GET", "/admin/rules", nil))).Items
	if len(rules) != 1 || rules[0].Who != "@familie" {
		t.Fatalf("rules = %+v", rules)
	}
	if slices.Contains(state.GetGroupMembers()["familie"], "anna") {
		t.Fatal("anna is still in the group")
	}
	for _, table := range []string{"user_prefs", "sessions", "account_profiles", "token_use"} {
		var n int
		if err := w.deps.DB.QueryRow(`SELECT COUNT(*) FROM `+table+` WHERE user_id = ?`, id).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s keeps %d rows of anna, %v", table, n, err)
		}
	}
	w.must(http.StatusNotFound, w.as(w.admin.Username, "DELETE", "/admin/users/anna", nil))
}

func TestTheRoleDecidesWhoMayShare(t *testing.T) {
	w := newWorld(t)
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
	w.must(http.StatusForbidden, w.as("anna", "POST", "/shares", link))
}
