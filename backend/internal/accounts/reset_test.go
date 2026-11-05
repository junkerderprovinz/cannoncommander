package accounts_test

import (
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// logged catches the reset codes that would go to the container log.
type logged struct {
	mu    sync.Mutex
	codes map[string][]string
}

func catchCodes(t *testing.T) *logged {
	l := &logged{codes: map[string][]string{}}
	t.Cleanup(accounts.CatchResetCodes(func(user, code string) {
		l.mu.Lock()
		defer l.mu.Unlock()
		l.codes[user] = append(l.codes[user], code)
	}))
	return l
}

func (l *logged) last(user string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	list := l.codes[user]
	if len(list) == 0 {
		return ""
	}
	return list[len(list)-1]
}

func TestAResetCodeGoesToTheLogOnly(t *testing.T) {
	w := newWorld(t)
	log := catchCodes(t)
	w.addUser("anna", "user", "/mnt/user")
	known := w.req("POST", "/auth/reset-code", map[string]string{"username": "anna"})
	unknown := w.req("POST", "/auth/reset-code", map[string]string{"username": "niemand"})
	if known.Code != http.StatusAccepted || unknown.Code != known.Code || unknown.Body.String() != known.Body.String() {
		t.Fatalf("the answer tells names apart: %d %q, %d %q", known.Code, known.Body, unknown.Code, unknown.Body)
	}
	code := log.last("anna")
	if len(code) != 14 || strings.Count(code, "-") != 2 || len(log.codes) != 1 {
		t.Fatalf("logged %v", log.codes)
	}
	w.req("POST", "/auth/reset-code", map[string]string{"username": "anna"})
	if len(log.codes["anna"]) != 1 {
		t.Fatal("asking again at once wrote another code")
	}
}

func TestResettingThePasswordClearsTheWayIn(t *testing.T) {
	w := newWorld(t)
	log := catchCodes(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	w.turnOnTotp("anna")
	tok := decode[accounts.NewToken](t, w.must(http.StatusCreated, w.as("anna", "POST", "/me/tokens", map[string]any{"password": strong, "name": "skript", "rights": "rw"})))
	w.req("POST", "/auth/reset-code", map[string]string{"username": "anna"})
	code := log.last("anna")

	w.must(http.StatusBadRequest, w.req("POST", "/auth/reset", map[string]string{"code": code, "password": "kurz"}))
	w.must(http.StatusBadRequest, w.req("POST", "/auth/reset", map[string]string{"code": "zu-kurz", "password": "ein-ganz-neues-passwort"}))
	rec := w.must(http.StatusOK, w.req("POST", "/auth/reset", map[string]string{"code": strings.ToLower(code), "password": "ein-ganz-neues-passwort"}))
	me := decode[accounts.Me](t, rec)
	if me.Name != "anna" || me.Twofa || me.Recovery != 0 || rec.Header().Get("Set-Cookie") == "" {
		t.Fatalf("after the reset: %+v, cookie %q", me, rec.Header().Get("Set-Cookie"))
	}
	if last := w.login.endAll[len(w.login.endAll)-1]; last.user != anna.ID || last.keep != "" {
		t.Fatalf("every session must end: %+v", last)
	}
	if _, _, ok := state.HashedTokenOwner(tok.Secret); ok || !state.IsTokenRevoked(tok.Secret) {
		t.Fatal("the API token survived the reset")
	}
	if err := w.passwordLogin("anna", "ein-ganz-neues-passwort", ""); err != nil {
		t.Fatalf("login after the reset: %v", err)
	}
	w.must(http.StatusBadRequest, w.req("POST", "/auth/reset", map[string]string{"code": code, "password": "noch-ein-neues-passwort"}))
}

func TestGuessingResetCodesIsCutShort(t *testing.T) {
	w := newWorld(t)
	log := catchCodes(t)
	w.addUser("anna", "user", "/mnt/user")
	w.req("POST", "/auth/reset-code", map[string]string{"username": "anna"})
	for range 5 {
		w.must(http.StatusBadRequest, w.req("POST", "/auth/reset", map[string]string{"code": "AAAA-AAAA-AAAA", "password": "ein-ganz-neues-passwort"}, "X-IP", "203.0.113.9"))
	}
	w.must(http.StatusTooManyRequests, w.req("POST", "/auth/reset", map[string]string{"code": log.last("anna"), "password": "ein-ganz-neues-passwort"}, "X-IP", "203.0.113.9"))
	w.must(http.StatusOK, w.req("POST", "/auth/reset", map[string]string{"code": log.last("anna"), "password": "ein-ganz-neues-passwort"}, "X-IP", "203.0.113.10"))
}
