package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

const day = 24 * time.Hour

func renewEnv(t *testing.T) *users.User {
	t.Helper()
	setupTestEnv(t)
	key := settings.Config.Auth.Key
	settings.Config.Auth.Key = "test-signing-key-for-renewed-sessions"
	t.Cleanup(func() { settings.Config.Auth.Key = key })
	accounts.UseLogin(sessionLogin{})
	return createTokenAuthUser(t, "bleiber", users.Permissions{Api: true})
}

// signedInAgo is a session token that signed in age ago and lasts life, as
// a sign in leaves it. stay is the box "stay signed in".
func signedInAgo(t *testing.T, u *users.User, age, life time.Duration, stay bool) string {
	t.Helper()
	at := time.Now().Add(-age)
	claims := jwt.MapClaims{"jti": utils.InsecureRandomIdentifier(8), "iss": auth.TokenIssuer, "iat": at.Unix(), "exp": at.Add(life).Unix(), "auth_time": at.Unix()}
	if stay {
		claims["stay"] = true
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(settings.Config.Auth.Key))
	if err != nil {
		t.Fatal(err)
	}
	if err := state.RegisterSessionToken(token, u.ID); err != nil {
		t.Fatal(err)
	}
	return token
}

func stayDays(t *testing.T, u *users.User, days int) {
	t.Helper()
	if _, err := state.MainDB().Exec(`INSERT INTO user_prefs (user_id, data, updated_at) VALUES (?, ?, ?)`,
		strconv.FormatUint(u.ID, 10), `{"stayDays":`+strconv.Itoa(days)+`}`, time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
}

func withCookie(token string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", http.NoBody)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: token})
	return req
}

// visit is a request of the page with the session cookie token, through the
// same door as every call to /api/v1.
func visit(token string) (*httptest.ResponseRecorder, error) {
	rec := httptest.NewRecorder()
	_, err := accounts.Track(state.MainDB(), authenticateV1)(rec, withCookie(token))
	return rec, err
}

// renewed returns the session cookie a response sets, or nil.
func renewed(rec *httptest.ResponseRecorder) *http.Cookie {
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookieName {
			return c
		}
	}
	return nil
}

func expiry(t *testing.T, token string) time.Time {
	t.Helper()
	var claims jwt.RegisteredClaims
	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil || claims.ExpiresAt == nil {
		t.Fatalf("token without expiry: %v", err)
	}
	return claims.ExpiresAt.Time
}

func TestAVisitRenewsASessionThatStaysSignedIn(t *testing.T) {
	u := renewEnv(t)
	stayDays(t, u, 7)
	old := signedInAgo(t, u, 2*day, 7*day, true)

	rec, err := visit(old)
	if err != nil {
		t.Fatal(err)
	}
	fresh := renewed(rec)
	if fresh == nil || fresh.Value == old {
		t.Fatalf("no new token, the response sets %v", rec.Header().Values("Set-Cookie"))
	}
	for what, end := range map[string]time.Time{"cookie": fresh.Expires, "token": expiry(t, fresh.Value)} {
		if left := time.Until(end); left < 7*day-time.Minute || left > 7*day {
			t.Errorf("the %s lasts another %v, want seven days", what, left)
		}
	}
	rec, err = visit(fresh.Value)
	if err != nil {
		t.Fatalf("the new token does not sign in: %v", err)
	}
	if again := renewed(rec); again != nil {
		t.Errorf("the new token was renewed at once: %v", again)
	}
	at, ok := sessionLogin{}.SignedInAt(withCookie(fresh.Value))
	if !ok || time.Since(at) < 2*day-time.Minute {
		t.Errorf("the renewed session signed in at %v, %v, want two days ago", at, ok)
	}

	// A request that left with the old token before the new one arrived still
	// passes and brings the same new token back.
	rec, err = visit(old)
	if err != nil {
		t.Fatalf("the old token stopped at once: %v", err)
	}
	if again := renewed(rec); again == nil || again.Value != fresh.Value {
		t.Errorf("the old token was renewed a second time: %v", again)
	}
}

func TestARenewedSessionKeepsItsPlaceInTheList(t *testing.T) {
	u := renewEnv(t)
	old := signedInAgo(t, u, 2*day, 30*day, true)
	db := state.MainDB()
	if _, err := db.Exec(`INSERT INTO sessions (id, user_id, token_hash, created_at, seen_at, user_agent, ip) VALUES ('s1', ?, ?, 1, 1, 'Firefox', '192.0.2.1')`,
		strconv.FormatUint(u.ID, 10), utils.HashSHA256(old)); err != nil {
		t.Fatal(err)
	}
	rec, err := visit(old)
	if err != nil {
		t.Fatal(err)
	}
	fresh := renewed(rec)
	if fresh == nil {
		t.Fatal("not renewed")
	}
	var id, hash string
	var n int
	if err := db.QueryRow(`SELECT COUNT(*), MIN(id), MIN(token_hash) FROM sessions WHERE user_id = ?`, strconv.FormatUint(u.ID, 10)).Scan(&n, &id, &hash); err != nil {
		t.Fatal(err)
	}
	if n != 1 || id != "s1" || hash != utils.HashSHA256(fresh.Value) {
		t.Errorf("the list holds %d sessions, %s with %s", n, id, hash)
	}
}

func TestASessionIsRenewedAtMostOnceADay(t *testing.T) {
	u := renewEnv(t)
	token := signedInAgo(t, u, 20*time.Hour, 30*day, true)
	rec, err := visit(token)
	if err != nil {
		t.Fatal(err)
	}
	if c := renewed(rec); c != nil {
		t.Errorf("a token of 20 hours was renewed: %v", c)
	}
}

func TestADaySessionIsRenewedWithinTheHour(t *testing.T) {
	u := renewEnv(t)
	stayDays(t, u, 1)
	rec, err := visit(signedInAgo(t, u, 90*time.Minute, day, true))
	if err != nil {
		t.Fatal(err)
	}
	fresh := renewed(rec)
	if fresh == nil {
		t.Fatal("a session of one day was not renewed after an hour and a half")
	}
	if left := time.Until(expiry(t, fresh.Value)); left < day-time.Minute {
		t.Errorf("the renewed session lasts %v", left)
	}
}

func TestASessionThatEndsWithTheBrowserIsNotRenewed(t *testing.T) {
	u := renewEnv(t)
	rec, err := visit(signedInAgo(t, u, 11*time.Hour, 12*time.Hour, false))
	if err != nil {
		t.Fatal(err)
	}
	if c := renewed(rec); c != nil {
		t.Errorf("a session without stay signed in was renewed: %v", c)
	}
}

func TestASessionLeftAloneForItsTimeEnds(t *testing.T) {
	u := renewEnv(t)
	if _, err := visit(signedInAgo(t, u, 7*day+time.Hour, 7*day, true)); err == nil {
		t.Error("a session of seven days still signs in after seven days without a visit")
	}
	rec, err := visit(signedInAgo(t, u, 7*day-time.Hour, 7*day, true))
	if err != nil {
		t.Fatalf("a session of seven days ended after six days and 23 hours: %v", err)
	}
	if renewed(rec) == nil {
		t.Error("the visit on the last day did not renew the session")
	}
}

func TestARevokedSessionStaysRevoked(t *testing.T) {
	u := renewEnv(t)
	login := sessionLogin{}
	renew := func() (old, fresh string) {
		t.Helper()
		old = signedInAgo(t, u, 2*day, 30*day, true)
		rec, err := visit(old)
		if err != nil {
			t.Fatal(err)
		}
		c := renewed(rec)
		if c == nil {
			t.Fatal("not renewed")
		}
		return old, c.Value
	}
	gone := func(why string, tokens ...string) {
		t.Helper()
		for _, token := range tokens {
			if rec, err := visit(token); err == nil {
				t.Errorf("%s: a token still signs in", why)
			} else if c := renewed(rec); c != nil {
				t.Errorf("%s: a token was renewed: %v", why, c)
			}
		}
	}

	ended := signedInAgo(t, u, 2*day, 30*day, true)
	if err := login.End(utils.HashSHA256(ended)); err != nil {
		t.Fatal(err)
	}
	gone("ended before the visit", ended)

	old, fresh := renew()
	if err := login.End(utils.HashSHA256(fresh)); err != nil {
		t.Fatal(err)
	}
	gone("ended from the list", old, fresh)

	old, fresh = renew()
	if err := login.Stop(httptest.NewRecorder(), withCookie(old)); err != nil {
		t.Fatal(err)
	}
	gone("signed out with the old token", old, fresh)

	old, fresh = renew()
	n, err := login.EndAll(u.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("ending every session counted %d, want the one", n)
	}
	gone("every session ended", old, fresh)
}
