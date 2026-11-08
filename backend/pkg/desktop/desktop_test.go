package desktop

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func guarded(t *testing.T) http.Handler {
	t.Helper()
	Use(nil, Options{Key: "s3cret", Port: 47321})
	t.Cleanup(func() { Use(nil, Options{}) })
	return Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))
}

func TestTheWindowEntersWithTheKeyAndKeepsItInACookie(t *testing.T) {
	h := guarded(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://127.0.0.1:47321"+EnterPath+"?key=s3cret", nil))
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Fatalf("got %d to %q", rec.Code, rec.Header().Get("Location"))
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Value != "s3cret" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie %+v", cookies)
	}

	req := httptest.NewRequest("GET", "http://127.0.0.1:47321/api/v1/system", nil)
	req.AddCookie(cookies[0])
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusTeapot {
		t.Fatalf("with the cookie: %d", rec.Code)
	}
}

func TestEverythingElseIsTurnedAway(t *testing.T) {
	h := guarded(t)
	for _, c := range []struct {
		name, url, cookie string
		want              int
	}{
		{"no cookie", "http://127.0.0.1:47321/api/v1/system", "", http.StatusForbidden},
		{"wrong cookie", "http://127.0.0.1:47321/api/v1/system", "guess", http.StatusForbidden},
		{"wrong key", "http://127.0.0.1:47321" + EnterPath + "?key=guess", "", http.StatusForbidden},
		{"a rebound name", "http://evil.example:47321/api/v1/system", "s3cret", http.StatusMisdirectedRequest},
		{"another port", "http://127.0.0.1:8080/api/v1/system", "s3cret", http.StatusMisdirectedRequest},
	} {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", c.url, nil)
			if c.cookie != "" {
				req.AddCookie(&http.Cookie{Name: CookieName, Value: c.cookie})
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != c.want {
				t.Fatalf("got %d, want %d", rec.Code, c.want)
			}
		})
	}
}

func TestLocalhostAndIPv6LoopbackCount(t *testing.T) {
	for _, host := range []string{"localhost:47321", "[::1]:47321", "127.0.0.1:47321"} {
		if !loopbackHost(host, 47321) {
			t.Errorf("%s was refused", host)
		}
	}
}

func TestTheContainerIsLeftAlone(t *testing.T) {
	Use(nil, Options{})
	h := Guard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "http://nas.local/api/v1/system", nil))
	if rec.Code != http.StatusTeapot || Active() {
		t.Fatalf("a server without the desktop app answered %d", rec.Code)
	}
}
