package accounts_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

// via follows a sign-in that b starts at start through p and back, and
// returns where the sign-in sent the browser in the end.
func (w *world) via(b *browser, p *signintest.Provider, start string) *url.URL {
	w.t.Helper()
	rec := b.do("GET", start, nil)
	if rec.Code != http.StatusFound {
		w.t.Fatalf("start %s: %d %s", start, rec.Code, rec.Body)
	}
	client := p.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(rec.Header().Get("Location"))
	if err != nil {
		w.t.Fatal(err)
	}
	resp.Body.Close()
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		w.t.Fatal(err)
	}
	done := b.do("GET", back.RequestURI(), nil)
	if done.Code != http.StatusFound {
		w.t.Fatalf("callback: %d %s", done.Code, done.Body)
	}
	to, _ := url.Parse(done.Header().Get("Location"))
	return to
}

// uiService adds p as a sign-in service through the sign-in's own form, as
// jdp, with role unless it is empty, and passes its test sign-in.
func (w *world) uiService(p *signintest.Provider, role string) string {
	w.t.Helper()
	body := map[string]any{"provider": "custom", "issuer": p.URL, "clientId": p.ClientID, "secret": p.Secret, "ca": string(p.CA)}
	if role != "" {
		body["role"] = role
	}
	added := decode[struct{ Service struct{ ID string } }](w.t, w.must(http.StatusOK, w.as("jdp", "POST", "/api/signin/v1/methods/oidc", body)))
	test := decode[struct{ URL string }](w.t, w.must(http.StatusOK, w.as("jdp", "POST", "/api/signin/v1/methods/oidc/"+added.Service.ID+"/test", map[string]string{"return": "/"})))
	b := w.browser()
	name, value, _ := strings.Cut(w.session("jdp"), "=")
	b.jar[name] = value
	p.Next(signintest.Person{Subject: "sub-jdp", Name: "jdp"})
	if to := w.via(b, p, test.URL); to.Query().Get("signin") != "testOk" {
		w.t.Fatalf("the test sign-in ended at %s", to)
	}
	return added.Service.ID
}

func TestANewcomerThroughAServiceOfTheAppGetsThatServicesRole(t *testing.T) {
	cases := []struct{ role, want string }{{"admin", "admin"}, {"user", "user"}, {"readonly", "readonly"}, {"", "user"}}
	for _, c := range cases {
		t.Run(c.want+"/"+c.role, func(t *testing.T) {
			w := newWorld(t).public()
			w.open()
			// Newcomers get the shares as their folder.
			settings.Config.Server.Sources[0].Config.DefaultEnabled = true
			p := signintest.NewProvider(t)
			id := w.uiService(p, c.role)
			p.Next(signintest.Person{Subject: "sub-erna", Name: "erna"})
			b := w.browser()
			w.via(b, p, "/signin/oidc/"+id+"?return=/")
			me, ok := w.meThrough(b)
			if !ok || me.Role != c.want {
				t.Fatalf("erna came in as %q", me.Role)
			}
			u, err := state.GetUserByUsername("erna")
			if err != nil {
				t.Fatal(err)
			}
			if u.Permissions.Admin != (c.want == "admin") {
				t.Fatalf("the row of erna is admin %v for the role %s", u.Permissions.Admin, c.want)
			}
			writes := false
			for _, s := range u.BackendScopes {
				writes = writes || s.Permissions.Create
			}
			if len(u.BackendScopes) == 0 || writes != (c.want != "readonly") {
				t.Fatalf("the scopes of erna %+v do not fit the role %s", u.BackendScopes, c.want)
			}
		})
	}
}

func TestTheSignInNamesUserAsTheRoleOfNewServices(t *testing.T) {
	w := newWorld(t).open()
	var roles struct {
		Roles       []string `json:"roles"`
		ServiceRole string   `json:"serviceRole"`
	}
	if err := json.Unmarshal(w.must(http.StatusOK, w.as("jdp", "GET", "/api/signin/v1/roles", nil)).Body.Bytes(), &roles); err != nil {
		t.Fatal(err)
	}
	if roles.ServiceRole != "user" || len(roles.Roles) != 3 {
		t.Fatalf("roles %+v", roles)
	}
}
