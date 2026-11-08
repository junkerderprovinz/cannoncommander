package accounts_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// fakeLogin stands in for the login system: the X-Token header is the
// session token and its value doubles as the hash, X-Api carries an API
// token, and X-IP the client address.
type fakeLogin struct {
	mu      sync.Mutex
	dead    map[string]bool
	ended   []string
	endAll  []endAll
	started []string
	kept    []time.Duration
	stopped []string
	// signedIn is when every session signed in, now when it is zero.
	signedIn time.Time
}

func (l *fakeLogin) SignedInAt(r *http.Request) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if r.Header.Get("X-Token") == "" {
		return time.Time{}, false
	}
	if l.signedIn.IsZero() {
		return time.Now(), true
	}
	return l.signedIn, true
}

// endAll is one call to EndAll.
type endAll struct {
	user uint64
	keep string
}

func (l *fakeLogin) Session(r *http.Request) (string, string, bool) {
	t := r.Header.Get("X-Token")
	return t, "192.168.10.20", t != ""
}

func (l *fakeLogin) Alive(hash string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return !l.dead[hash]
}

func (l *fakeLogin) End(hash string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.dead[hash] = true
	l.ended = append(l.ended, hash)
	return nil
}

// EndAll pretends every user has two other sessions.
func (l *fakeLogin) EndAll(userID uint64, keep string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.endAll = append(l.endAll, endAll{userID, keep})
	return 2, nil
}

func (l *fakeLogin) APIToken(r *http.Request) (string, bool) {
	t := r.Header.Get("X-Api")
	return t, t != ""
}

func (l *fakeLogin) Start(w http.ResponseWriter, _ *http.Request, u *users.User, keep time.Duration) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.started = append(l.started, u.Username)
	l.kept = append(l.kept, keep)
	w.Header().Set("Set-Cookie", "session=fresh")
	return nil
}

func (l *fakeLogin) Renew(http.ResponseWriter, *http.Request, *users.User) (string, bool) {
	return "", false
}

func (l *fakeLogin) Stop(w http.ResponseWriter, r *http.Request) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.stopped = append(l.stopped, r.Header.Get("X-Token"))
	w.Header().Set("Set-Cookie", "session=; Max-Age=0")
	return nil
}

func (l *fakeLogin) ClientIP(r *http.Request) string {
	if ip := r.Header.Get("X-IP"); ip != "" {
		return ip
	}
	return "192.0.2.1"
}

func (l *fakeLogin) PasskeySite(*http.Request) (string, string) {
	return "cannon.lan", "cannon.lan:8443"
}

type fixture struct {
	t     *testing.T
	h     http.Handler
	db    *sql.DB
	login *fakeLogin
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "main.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if _, err := migrate.Apply(context.Background(), db, migrate.Main); err != nil {
		t.Fatal(err)
	}
	login := &fakeLogin{dead: map[string]bool{}}
	accounts.UseLogin(login)
	people := map[string]*users.User{"jdp": {ID: 1}, "anna": {ID: 2}}
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	r := v1.NewRouter(v1.Deps{DB: db}, accounts.Track(db, auth))
	accounts.Routes(r)
	h, err := r.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return &fixture{t: t, h: h, db: db, login: login}
}

func (f *fixture) do(user, token, method, target string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(method, target, nil)
	req.Header.Set("X-User", user)
	req.Header.Set("X-Token", token)
	req.Header.Set("User-Agent", "Firefox auf "+token)
	rec := httptest.NewRecorder()
	f.h.ServeHTTP(rec, req)
	return rec
}

func (f *fixture) sessions(user, token string) []accounts.Session {
	f.t.Helper()
	rec := f.do(user, token, "GET", "/me/sessions")
	if rec.Code != http.StatusOK {
		f.t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	var body struct{ Items []accounts.Session }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		f.t.Fatal(err)
	}
	return body.Items
}

func agents(list []accounts.Session) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.UserAgent)
	}
	slices.Sort(out)
	return out
}

func TestEverySessionThatCallsShowsUp(t *testing.T) {
	f := newFixture(t)
	f.do("jdp", "desk-1", "GET", "/me/sessions")
	f.do("jdp", "phone-1", "GET", "/me/sessions")
	f.do("anna", "anna-1", "GET", "/me/sessions")

	list := f.sessions("jdp", "desk-1")
	if got := agents(list); !slices.Equal(got, []string{"Firefox auf desk-1", "Firefox auf phone-1"}) {
		t.Fatalf("jdp sees %v", got)
	}
	for _, s := range list {
		if s.Current != (s.UserAgent == "Firefox auf desk-1") || s.IP != "192.168.10.20" || s.ID == "" || s.Created.IsZero() {
			t.Errorf("session %+v", s)
		}
	}
	if got := agents(f.sessions("anna", "anna-1")); !slices.Equal(got, []string{"Firefox auf anna-1"}) {
		t.Errorf("anna sees %v", got)
	}
}

func TestEndingASessionLogsItOut(t *testing.T) {
	f := newFixture(t)
	f.do("jdp", "desk-2", "GET", "/me/sessions")
	f.do("jdp", "phone-2", "GET", "/me/sessions")
	var phone accounts.Session
	for _, s := range f.sessions("jdp", "desk-2") {
		if !s.Current {
			phone = s
		}
	}

	if rec := f.do("anna", "anna-2", "DELETE", "/me/sessions/"+phone.ID); rec.Code != http.StatusNotFound {
		t.Fatalf("anna ended jdp's session: %d", rec.Code)
	}
	if rec := f.do("jdp", "desk-2", "DELETE", "/me/sessions/"+phone.ID); rec.Code != http.StatusNoContent {
		t.Fatalf("end: %d %s", rec.Code, rec.Body)
	}
	if !slices.Equal(f.login.ended, []string{"phone-2"}) {
		t.Errorf("ended %v", f.login.ended)
	}
	if got := agents(f.sessions("jdp", "desk-2")); !slices.Equal(got, []string{"Firefox auf desk-2"}) {
		t.Errorf("left %v", got)
	}
}

func TestSessionsThatEndedElsewhereLeaveTheList(t *testing.T) {
	f := newFixture(t)
	f.do("jdp", "desk-3", "GET", "/me/sessions")
	f.do("jdp", "laptop-3", "GET", "/me/sessions")
	f.login.dead["laptop-3"] = true
	if got := agents(f.sessions("jdp", "desk-3")); !slices.Equal(got, []string{"Firefox auf desk-3"}) {
		t.Errorf("list %v", got)
	}
	var n int
	if err := f.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE token_hash = 'laptop-3'`).Scan(&n); err != nil || n != 0 {
		t.Errorf("row kept: %d %v", n, err)
	}
}

func TestRequestsWithoutASessionTokenAreNotListed(t *testing.T) {
	f := newFixture(t)
	f.do("jdp", "", "GET", "/me/sessions")
	if got := f.sessions("jdp", ""); len(got) != 0 {
		t.Errorf("listed %+v", got)
	}
}
