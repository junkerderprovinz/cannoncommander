package web

import (
	"context"
	"net/http"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// mountSignIn opens the sign-in and serves its contract, its pages and the
// OIDC paths of config.yaml. It returns what finds the visitor of every
// request. Under noauth, as in the desktop app, nobody signs in, so nothing
// opens and the contract only says that the app is open.
func mountSignIn(ctx context.Context, router *http.ServeMux, deps v1.Deps) (func(http.Handler) http.Handler, error) {
	base := settings.Config.Http.BaseURL
	if settings.Config.Auth.Methods.NoAuth {
		router.HandleFunc("GET "+base+"api/signin/v1/options", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"open":true}`))
		})
		return func(next http.Handler) http.Handler { return next }, nil
	}
	auth, err := accounts.Open(ctx, deps)
	if err != nil {
		return nil, err
	}
	h := accounts.Handler(auth)
	for _, p := range []string{"api/signin/v1/", "signin/", "api/auth/oidc/"} {
		router.Handle(base+p, h)
	}
	dav := base + "dav"
	return func(next http.Handler) http.Handler {
		guarded := auth.Guard(clearOldCookie(next))
		// WebDAV reads Basic credentials alone, so no cookie counts there.
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == dav || strings.HasPrefix(r.URL.Path, dav+"/") {
				next.ServeHTTP(w, r)
				return
			}
			guarded.ServeHTTP(w, r)
		})
	}, nil
}

// clearOldCookie drops the session cookie of the version before once the
// browser has a session of the sign-in. That cookie went to every subdomain
// and would sign the browser back in after going back to that version.
func clearOldCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if v, ok := signin.From(r.Context()); ok && v.Session() != "" {
			if _, err := r.Cookie(accounts.OldCookie); err == nil {
				http.SetCookie(w, &http.Cookie{
					Name: accounts.OldCookie, Value: "", Domain: strings.Split(requestHost(r), ":")[0], Path: "/",
					MaxAge: -1, HttpOnly: true, Secure: requestScheme(r) == "https",
				})
			}
		}
		next.ServeHTTP(w, r)
	})
}

// terminalGate tells the terminal how a request signed in and where it came
// from.
type terminalGate struct {
	guard *http.CrossOriginProtection
}

func (g terminalGate) SameOrigin(r *http.Request) error { return sameOrigin(g.guard, r) }

func (terminalGate) Session(r *http.Request) (string, bool) { return accounts.SessionOf(r) }

func (terminalGate) Alive(id string) bool { return accounts.Alive(id) }

func (terminalGate) APIToken(r *http.Request) bool { return accounts.ByToken(r) }

func (terminalGate) Open() bool { return state.OpenAccess() }
