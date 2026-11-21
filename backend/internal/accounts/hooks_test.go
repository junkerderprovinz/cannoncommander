package accounts_test

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// endings records what other packages heard through OnSessionsEnd and
// OnTokensEnd, by account.
var endings = struct {
	sync.Mutex
	sessions, tokens map[uint64]int
}{sessions: map[uint64]int{}, tokens: map[uint64]int{}}

func init() {
	accounts.OnSessionsEnd(func(_ context.Context, _ *sql.DB, id uint64) error {
		endings.Lock()
		defer endings.Unlock()
		endings.sessions[id]++
		return nil
	})
	accounts.OnTokensEnd(func(_ context.Context, _ *sql.DB, id uint64) error {
		endings.Lock()
		defer endings.Unlock()
		endings.tokens[id]++
		return nil
	})
}

func heard(id uint64) (sessions, tokens int) {
	endings.Lock()
	defer endings.Unlock()
	return endings.sessions[id], endings.tokens[id]
}

func TestAnAccountTheSignInMakesGetsARowWithANumericID(t *testing.T) {
	w := newWorld(t).open()
	rec := w.must(http.StatusOK, w.as("jdp", "POST", "/api/signin/v1/accounts", map[string]string{"name": "gast", "password": strong, "role": "readonly"}))
	acc := decode[struct{ Account signin.Account }](t, rec).Account
	id, err := strconv.ParseUint(acc.ID, 10, 64)
	if err != nil {
		t.Fatalf("the sign-in gave the id %q", acc.ID)
	}
	u, err := state.GetUserByID(id)
	if err != nil || u.Username != "gast" || u.Permissions.Admin || u.Permissions.Share {
		t.Fatalf("the row of gast: %+v, %v", u, err)
	}
	w.must(http.StatusOK, w.as("gast", "GET", "/me", nil))
}

func TestARowLeftByAFailedCreateGivesWayToTheNextOne(t *testing.T) {
	w := newWorld(t).open()
	left := users.User{FrontendUser: users.FrontendUser{Username: "bert", LoginMethod: users.LoginMethodPassword}}
	if err := state.CreateUser(&left, ""); err != nil {
		t.Fatal(err)
	}
	w.must(http.StatusOK, w.as("jdp", "POST", "/api/signin/v1/accounts", map[string]string{"name": "bert", "password": strong, "role": "user"}))
	if _, err := state.GetUserByID(left.ID); err == nil {
		t.Fatal("the row nobody could sign in with is still there")
	}
	w.must(http.StatusOK, w.as("bert", "GET", "/me", nil))
}

func TestEndingsReachMCPGrantsAndKeys(t *testing.T) {
	w := newWorld(t).open()
	anna := w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.as("anna", "DELETE", "/api/signin/v1/me/sessions", nil))
	if s, k := heard(anna.ID); s != 1 || k != 0 {
		t.Fatalf("signing out everywhere: grants %d, keys %d", s, k)
	}
	delete(w.cookies, "anna")
	w.must(http.StatusOK, w.as("anna", "PUT", "/api/signin/v1/me/password", map[string]string{"current": strong, "next": "noch-ein-langes-passwort"}))
	if s, k := heard(anna.ID); s != 2 || k != 0 {
		t.Fatalf("a new password: grants %d, keys %d", s, k)
	}
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"blocked": true}))
	if s, k := heard(anna.ID); s != 3 || k != 1 {
		t.Fatalf("blocking: grants %d, keys %d", s, k)
	}
}

func TestTheRemovalRunsAgainUntilItIsDone(t *testing.T) {
	w := newWorld(t).open()
	anna := w.addUser("anna", "user", "/mnt/user")
	ended := accounts.Ended(w.deps)
	id := strconv.FormatUint(anna.ID, 10)
	for range 2 {
		if err := ended(t.Context(), id, signin.EndRemoved); err != nil {
			t.Fatalf("a removal that is done already failed: %v", err)
		}
	}
	if _, err := state.GetUserByID(anna.ID); err == nil {
		t.Fatal("the row is still there")
	}
}

func TestSignInAdminRoutesNeedAnAdminInTheUsersTable(t *testing.T) {
	w := newWorld(t).open()
	anna := w.addUser("anna", "admin", "/mnt/user")
	w.must(http.StatusOK, w.as("anna", "GET", "/api/signin/v1/accounts", nil))
	anna.Permissions.Admin = false
	if err := state.UpdateUser(&anna, "", "permissions"); err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"/api/signin/v1/accounts", "/api/signin/v1/log", "/api/signin/v1/methods/oidc", "/api/signin/v1/settings"} {
		if rec := w.as("anna", "GET", route, nil); rec.Code != http.StatusForbidden {
			t.Errorf("%s for a sign-in admin who is no admin here: %d", route, rec.Code)
		}
	}
	w.must(http.StatusForbidden, w.as("anna", "GET", "/admin/users", nil))
}

func TestTheOwnerKeepsItsRoleAndCannotBeBlocked(t *testing.T) {
	w := newBareWorld(t)
	admin := w.oldUser("admin", "", true)
	w.open()
	b := w.browser()
	w.must(http.StatusOK, b.do("PUT", "/api/signin/v1/me/password", map[string]string{"next": strong}))
	w.cookies["admin"] = b.session()
	w.admin = admin
	w.addUser("zweite", "admin", "/mnt/user")
	for _, change := range []map[string]any{{"role": "user"}, {"blocked": true}} {
		if rec := w.as("zweite", "PATCH", "/admin/users/admin", change); rec.Code != http.StatusForbidden || code(t, rec) != "accountOwner" {
			t.Errorf("%v on the owner: %d %s", change, rec.Code, rec.Body)
		}
	}
	w.must(http.StatusForbidden, w.as("zweite", "DELETE", "/admin/users/admin", nil))
}

func TestAnAccountWithoutARowGetsNobodysRights(t *testing.T) {
	w := newWorld(t).open()
	anna := w.addUser("anna", "admin", "/mnt/user")
	session := w.session("anna")
	if err := state.DeleteUser(anna.ID); err != nil {
		t.Fatal(err)
	}
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, header{"Cookie", session}))
}

func TestConfirmingTheOwnerWantsAFreshSignIn(t *testing.T) {
	w := newWorld(t)
	old := w.oldSession(w.admin, time.Now().Add(-time.Hour))
	w.open()
	confirm := func(cookie string) error {
		var err error
		w.auth.Guard(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { err = accounts.ConfirmOwner(r) })).
			ServeHTTP(httptest.NewRecorder(), func() *http.Request {
				r := httptest.NewRequest("POST", "http://"+host+"/api/v1/mcp/keys", nil)
				r.Header.Set("Cookie", cookie)
				return r
			}())
		return err
	}
	stale := sessionCookie(t, w.must(http.StatusOK, w.req("GET", "/me", nil, oldCookie(old))))
	if err := confirm(stale); err == nil {
		t.Fatal("a sign-in from an hour ago made a lasting key")
	}
	if err := confirm(w.session("jdp")); err != nil {
		t.Fatalf("a fresh sign-in: %v", err)
	}
}

func TestChangesFromBothSidesDoNotWaitForEachOther(t *testing.T) {
	w := newWorld(t).open()
	w.must(http.StatusCreated, w.as("jdp", "POST", "/admin/groups", map[string]any{"name": "familie"}))
	w.addUser("anna", "user", "/mnt/user")
	done := make(chan struct{})
	go func() {
		defer close(done)
		var wg sync.WaitGroup
		for i := range 10 {
			wg.Add(2)
			go func() {
				defer wg.Done()
				role := []string{"readonly", "user"}[i%2]
				w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"role": role})
			}()
			go func() {
				defer wg.Done()
				w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"groups": []string{"familie"}, "full": "Anna " + strconv.Itoa(i)})
			}()
		}
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatal("changes through the sign-in and to the users table wait for each other")
	}
}

func TestLinksOfABlockedOwnerStopServing(t *testing.T) {
	w := newWorld(t).open()
	w.addUser("anna", "user", "/mnt/user")
	link := decode[struct{ Slug string }](t, w.must(http.StatusCreated, w.as("anna", "POST", "/shares", map[string]any{"paths": []string{"/mnt/user/fotos"}})))
	w.must(http.StatusOK, w.req("GET", "/s/"+link.Slug, nil))
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"blocked": true}))
	if rec := w.req("GET", "/s/"+link.Slug, nil); rec.Code == http.StatusOK {
		t.Fatal("the link of a blocked owner still serves")
	}
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"blocked": false}))
	w.must(http.StatusOK, w.req("GET", "/s/"+link.Slug, nil))
}
