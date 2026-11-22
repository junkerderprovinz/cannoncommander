package accounts_test

import (
	"net/http"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

func TestAServiceAddedWhileOpenClosesTheServerAndEndsWhatWasMadeOpen(t *testing.T) {
	w := newBareWorld(t)
	jdp := w.oldUser("jdp", "", true, users.BackendScope{Path: w.share, Scope: "/"})
	w.public()
	w.open()
	p := signintest.NewProvider(t)
	body := map[string]any{"provider": "custom", "issuer": p.URL, "clientId": p.ClientID, "secret": p.Secret, "ca": string(p.CA)}
	added := decode[struct{ Service struct{ ID string } }](t, w.must(http.StatusOK, w.req("POST", "/api/signin/v1/methods/oidc", body)))
	test := decode[struct{ URL string }](t, w.must(http.StatusOK, w.req("POST", "/api/signin/v1/methods/oidc/"+added.Service.ID+"/test", map[string]string{"return": "/"})))
	_, tokensBefore := heard(jdp.ID)
	p.Next(signintest.Person{Subject: "sub-jdp", Name: "jdp"})
	b := w.browser()
	if to := w.via(b, p, test.URL); to.Query().Get("signin") != "testOk" {
		t.Fatalf("the test sign-in ended at %s", to)
	}
	if state.OpenAccess() {
		t.Fatal("the server is still open with a service on")
	}
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil))
	if _, tokens := heard(jdp.ID); tokens == tokensBefore {
		t.Fatal("the keys made while open outlast the closing")
	}
	acc := decode[struct{ PwChanged *string }](t, w.must(http.StatusOK, b.do("GET", "/me", nil)))
	if acc.PwChanged != nil {
		t.Fatalf("closing through a service set a password date: %s", *acc.PwChanged)
	}
}

func TestATokenMadeWhileNobodyOwnsTheServerMakesTheOwnerARow(t *testing.T) {
	w := newBareWorld(t)
	w.oldUser("kurt", "", false, users.BackendScope{Path: w.share, Scope: "/"})
	w.open()
	tok := decode[struct{ Value string }](t, w.must(http.StatusOK, w.req("POST", "/api/signin/v1/me/tokens", map[string]any{"name": "skript", "scopes": []string{"all"}})))
	owner, err := state.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("the owner made for the token has no row: %v", err)
	}
	if !owner.Permissions.Admin {
		t.Fatal("the owner's row is no admin")
	}
	w.must(http.StatusOK, w.req("GET", "/me", nil, header{"Authorization", "Bearer " + tok.Value}))
}

func TestAnOpenServerActsAsTheOwnerAfterARename(t *testing.T) {
	w := newBareWorld(t)
	admin := w.oldUser("admin", "", true, users.BackendScope{Path: w.share, Scope: "/"})
	w.oldUser("anton", "", true, users.BackendScope{Path: w.share, Scope: "/"})
	w.open()
	w.must(http.StatusOK, w.req("PATCH", "/admin/users/admin", map[string]any{"name": "chef"}))
	if u, err := state.ResolveNoAuthUser(); err != nil || u.ID != admin.ID {
		t.Fatalf("the open server acts as %q: %v", u.Username, err)
	}
	w.must(http.StatusOK, w.req("GET", "/admin/users", nil))
	_, before := heard(admin.ID)
	w.must(http.StatusOK, w.req("PUT", "/api/signin/v1/me/password", map[string]string{"next": strong}))
	if _, after := heard(admin.ID); after == before {
		t.Fatal("what the open server made outlasts the first password")
	}
}
