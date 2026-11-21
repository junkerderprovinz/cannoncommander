package accounts_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

// withProvider puts a provider that runs in the test into config.yaml.
func (w *world) withProvider(adminGroup string, userGroups ...string) *signintest.Provider {
	w.t.Helper()
	p := signintest.NewProvider(w.t)
	ca := filepath.Join(w.base, "ca.pem")
	if err := os.WriteFile(ca, p.CA, 0o600); err != nil {
		w.t.Fatal(err)
	}
	o := &settings.Config.Auth.Methods.OidcAuth
	o.Enabled, o.IssuerUrl, o.ClientID, o.ClientSecret, o.CaFile = true, p.URL, p.ClientID, p.Secret, ca
	o.UserIdentifier, o.GroupsClaim, o.Scopes = "preferred_username", "groups", "openid profile"
	o.AdminGroup, o.UserGroups = adminGroup, userGroups
	w.public()
	return p
}

// throughProvider signs person in through the OIDC of config.yaml, on the
// paths the version before used, and returns the browser and where the
// sign-in sent it back to.
func (w *world) throughProvider(p *signintest.Provider, person signintest.Person) (*browser, *url.URL) {
	w.t.Helper()
	p.Next(person)
	b := w.browser()
	start := b.do("GET", "/api/auth/oidc/login?redirect=/files", nil)
	if start.Code != http.StatusFound {
		w.t.Fatalf("start: %d %s", start.Code, start.Body)
	}
	authorize := start.Header().Get("Location")
	if !strings.Contains(authorize, url.QueryEscape("https://"+host+"/api/auth/oidc/callback")) {
		w.t.Fatalf("the provider is not sent back on the path it knows: %s", authorize)
	}
	client := p.Client()
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := client.Get(authorize)
	if err != nil {
		w.t.Fatal(err)
	}
	resp.Body.Close()
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || back.Path != "/api/auth/oidc/callback" {
		w.t.Fatalf("the provider answered %s", resp.Header.Get("Location"))
	}
	done := b.do("GET", back.RequestURI(), nil)
	if done.Code != http.StatusFound {
		w.t.Fatalf("callback: %d %s", done.Code, done.Body)
	}
	to, _ := url.Parse(done.Header().Get("Location"))
	return b, to
}

func (w *world) meThrough(b *browser) (accounts.Me, bool) {
	w.t.Helper()
	if b.session() == "" {
		return accounts.Me{}, false
	}
	rec := w.req("GET", "/me", nil, header{"Cookie", b.session()})
	return decode[accounts.Me](w.t, rec), rec.Code == http.StatusOK
}

func TestANewcomerThroughTheConfigIsAUserUnlessInTheAdminGroup(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("")
	w.open()
	b, to := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna"})
	me, ok := w.meThrough(b)
	if !ok || me.Name != "erna" || me.Role == "admin" || to.Path != "/files" {
		t.Fatalf("erna: %+v, back to %s", me, to)
	}
	u, err := state.GetUserByUsername("erna")
	if err != nil || u.LoginMethod != users.LoginMethodOidc || u.Permissions.Admin {
		t.Fatalf("the row of erna: %+v, %v", u, err)
	}
	var bound string
	if err := w.deps.DB.QueryRow(`SELECT subject FROM oidc_identities WHERE user_id = ?`, strconv.FormatUint(u.ID, 10)).Scan(&bound); err != nil || bound != "s-1" {
		t.Fatalf("the version before cannot find erna's binding: %q, %v", bound, err)
	}
}

func TestTheConfigServiceRefusesSomeoneOutsideUserGroups(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("", "familie")
	w.open()
	b, to := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "fremd", Groups: []string{"nachbarn"}})
	if b.session() != "" || to.Query().Get("signin") != "notAdmitted" {
		t.Fatalf("someone outside userGroups got in: back to %s", to)
	}
	if _, err := state.GetUserByUsername("fremd"); err == nil {
		t.Fatal("a refused newcomer got an account")
	}
	b, _ = w.throughProvider(p, signintest.Person{Subject: "s-2", Name: "anna", Groups: []string{"familie"}})
	if _, ok := w.meThrough(b); !ok {
		t.Fatal("a member of userGroups did not get in")
	}
}

func TestAnAccountOutsideUserGroupsLosesItsSessionsAtOnce(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("", "familie")
	w.open()
	first, _ := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "anna", Groups: []string{"familie"}})
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "anna", Groups: []string{"ex"}})
	if _, ok := w.meThrough(first); ok {
		t.Fatal("the session of an account the provider took out of userGroups still works")
	}
}

func TestTheAdminGroupDecidesTheRoleOnEverySignIn(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("chefs")
	w.open()
	b, _ := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{"chefs"}})
	if me, _ := w.meThrough(b); me.Role != "admin" {
		t.Fatalf("a newcomer in the admin group is %q", me.Role)
	}
	b, _ = w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{"andere"}})
	if me, _ := w.meThrough(b); me.Role != "user" {
		t.Fatalf("erna out of the admin group is %q", me.Role)
	}
	b, _ = w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna"})
	if me, _ := w.meThrough(b); me.Role != "user" {
		t.Fatalf("a sign-in without groups changed the role to %q", me.Role)
	}
}

// A group an admin renamed is the admin's, as one made by hand.
func TestAProviderGroupOnlyChangesMembershipsItGave(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("")
	w.open()
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{"fotos", "filme"}})
	w.must(http.StatusCreated, w.as("jdp", "POST", "/admin/groups", map[string]any{"name": "handarbeit", "members": []string{"erna"}}))
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/groups/filme", map[string]any{"name": "kino"}))
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{"fotos"}})
	got := state.GetUserGroups("erna")
	slices.Sort(got)
	if !slices.Equal(got, []string{"fotos", "handarbeit", "kino"}) {
		t.Fatalf("erna's groups: %v", got)
	}
}

func TestAnUnboundOIDCAccountIsClaimedOnceByItsName(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("")
	bert := w.oldUser("bert", "", false)
	bert.LoginMethod = users.LoginMethodOidc
	if err := state.UpdateUser(&bert, "", "loginMethod"); err != nil {
		t.Fatal(err)
	}
	w.oldUser("carl", strong, false)
	w.oldUser("dora", "", false)
	w.open()

	b, _ := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "bert"})
	if me, ok := w.meThrough(b); !ok || me.ID != strconv.FormatUint(bert.ID, 10) {
		t.Fatalf("bert's first sign-in did not claim the account: %+v", me)
	}
	if b, to := w.throughProvider(p, signintest.Person{Subject: "s-2", Name: "bert"}); b.session() != "" {
		t.Fatalf("a second person named bert got in, back to %s", to)
	}
	for _, name := range []string{"carl", "dora"} {
		if b, _ := w.throughProvider(p, signintest.Person{Subject: "s-" + name, Name: name}); b.session() != "" {
			t.Fatalf("a person named like the password account %s took it", name)
		}
	}
}

func TestOIDCFromTheConfigKeepsTheServerClosedWithoutPasswords(t *testing.T) {
	w := newBareWorld(t)
	w.withProvider("")
	w.oldUser(settings.PasswordAdminUsername(), "", true)
	w.open()
	if w.auth.Anyone() || state.OpenAccess() {
		t.Fatal("a server behind its OIDC came over open")
	}
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil))
}

func TestPasswordsSwitchedOffStayOffAfterTheUpgrade(t *testing.T) {
	w := newWorld(t)
	w.withProvider("")
	settings.Config.Auth.Methods.PasswordAuth.Enabled = false
	w.open()
	if w.account(w.admin).Password {
		t.Fatal("a password came over although passwords are off")
	}
	if rec := w.req("POST", "/api/signin/v1/login", map[string]string{"name": "jdp", "password": strong}); rec.Code != http.StatusForbidden || code(t, rec) != "passwordsOff" {
		t.Fatalf("a password sign-in with passwords off: %d %s", rec.Code, rec.Body)
	}
}

func TestStartupNamesWhyTheConfigServiceCannotRun(t *testing.T) {
	w := newWorld(t)
	w.withProvider("")
	settings.Config.Http.ExternalUrl = ""
	_, err := accounts.Open(context.Background(), w.deps)
	if err == nil || !strings.Contains(err.Error(), "http.externalUrl") || !strings.Contains(err.Error(), "/api/auth/oidc/callback") {
		t.Fatalf("no line about the address: %v", err)
	}
	w.public()
	settings.Config.Auth.Methods.OidcAuth.DisableVerifyTLS = true
	if _, err := accounts.Open(context.Background(), w.deps); err == nil || !strings.Contains(err.Error(), "caFile") {
		t.Fatalf("no line about caFile: %v", err)
	}
}

// resetCodes catches the reset code the sign-in writes to the log.
type resetCodes struct {
	slog.Handler
	mu    *sync.Mutex
	codes *[]string
}

func (h resetCodes) Handle(ctx context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == "code" {
			h.mu.Lock()
			*h.codes = append(*h.codes, a.Value.String())
			h.mu.Unlock()
		}
		return true
	})
	return nil
}

func (h resetCodes) Enabled(context.Context, slog.Level) bool { return true }

func TestAResetGivesAnOIDCRowAPasswordNobodyKnows(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("")
	var codes []string
	saved := slog.Default()
	slog.SetDefault(slog.New(resetCodes{Handler: saved.Handler(), mu: &sync.Mutex{}, codes: &codes}))
	t.Cleanup(func() { slog.SetDefault(saved) })
	w.open()
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna"})

	w.must(http.StatusOK, w.req("POST", "/api/signin/v1/reset-code", map[string]string{}))
	if len(codes) != 1 {
		t.Fatalf("reset codes in the log: %v", codes)
	}
	w.must(http.StatusOK, w.req("POST", "/api/signin/v1/reset", map[string]string{"code": codes[0], "password": strong, "name": "erna"}))
	u, err := state.GetUserByUsername("erna")
	if err != nil || u.LoginMethod != users.LoginMethodPassword || u.Password == "" || utils.CheckPwd(strong, u.Password) == nil {
		t.Fatalf("after the reset the version before would bind erna by name or let her new password in: %s %q", u.LoginMethod, u.Password)
	}
}

func TestLeavingEveryGroupEndsTheProviderMembershipsAndAMissingClaimChangesNothing(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("")
	w.open()
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{"fotos"}})
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna"})
	if got := state.GetUserGroups("erna"); !slices.Equal(got, []string{"fotos"}) {
		t.Fatalf("a sign-in without a groups claim changed the groups: %v", got)
	}
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{}})
	if got := state.GetUserGroups("erna"); len(got) != 0 {
		t.Fatalf("erna left every group and kept %v", got)
	}
}

func TestLeavingEveryGroupIsRefusedUnderUserGroups(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("", "familie")
	w.open()
	first, _ := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "anna", Groups: []string{"familie"}})
	if b, to := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "anna", Groups: []string{}}); b.session() != "" || to.Query().Get("signin") != "notAdmitted" {
		t.Fatalf("anna without any group got in, back to %s", to)
	}
	if _, ok := w.meThrough(first); ok {
		t.Fatal("the earlier session of anna still works")
	}
}

func TestLeavingEveryGroupTakesTheAdminRoleAndAMissingClaimLeavesIt(t *testing.T) {
	w := newWorld(t)
	p := w.withProvider("chefs")
	w.open()
	w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{"chefs"}})
	b, _ := w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna"})
	if me, _ := w.meThrough(b); me.Role != "admin" {
		t.Fatalf("a sign-in without a groups claim made erna %q", me.Role)
	}
	b, _ = w.throughProvider(p, signintest.Person{Subject: "s-1", Name: "erna", Groups: []string{}})
	if me, _ := w.meThrough(b); me.Role != "user" {
		t.Fatalf("erna left every group and is %q", me.Role)
	}
}

func TestAServerBehindItsOIDCGetsNoOwner(t *testing.T) {
	w := newBareWorld(t)
	w.withProvider("")
	admin := w.oldUser(settings.PasswordAdminUsername(), "", true)
	w.open()
	if w.account(admin).Owner {
		t.Fatal("the admin of a server behind its OIDC became the owner")
	}
}

func TestWebDAVTakesNoPasswordWhilePasswordsAreOff(t *testing.T) {
	w := newWorld(t)
	w.withProvider("")
	w.open()
	r := httptest.NewRequest("PROPFIND", "http://"+host+"/dav/", nil)
	r.SetBasicAuth("jdp", strong)
	if _, err := accounts.BasicLogin(r, "jdp", strong); err != nil {
		t.Fatalf("with passwords on: %v", err)
	}
	settings.Config.Auth.Methods.PasswordAuth.Enabled = false
	if _, err := accounts.BasicLogin(r, "jdp", strong); err == nil {
		t.Fatal("WebDAV took a password while passwords are off")
	}
}
