package accounts

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

var errSignInNeeded = v1.Coded(http.StatusUnauthorized, "signInNeeded", "login required")

// Authenticate is the authenticator of /api/v1: the user of the account the
// sign-in found for r, narrowed to what its token allows. An account the
// users table does not know gets nobody's rights.
func Authenticate(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
	v, ok := signin.From(r.Context())
	if !ok || v.Via == signin.ViaAnyone {
		if !state.OpenAccess() {
			return nil, errSignInNeeded
		}
		u, err := state.ResolveNoAuthUser()
		if err != nil {
			return nil, err
		}
		return &u, nil
	}
	u, err := userOf(v)
	if err != nil {
		return nil, errSignInNeeded
	}
	if v.Via == signin.ViaToken {
		readOnly, err := TokenRights(u, v.Scopes)
		if err != nil {
			return nil, v1.Coded(http.StatusForbidden, "denied", err.Error())
		}
		if readOnly && !safe(r.Method) {
			return nil, v1.Coded(http.StatusForbidden, "tokenReadOnly", "this API token may only read")
		}
	}
	return u, nil
}

func safe(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// visitor returns how r signed in, with ok false without a sign-in.
func visitor(r *http.Request) (signin.Visitor, bool) { return signin.From(r.Context()) }

// SessionOf returns the sign-in's id of the session r came with. ok is false
// for a token and for a request without a session.
func SessionOf(r *http.Request) (string, bool) {
	v, ok := visitor(r)
	return v.Session(), ok && v.Session() != ""
}

// ByToken reports whether r came with an API token.
func ByToken(r *http.Request) bool {
	v, ok := visitor(r)
	return ok && v.Via == signin.ViaToken
}

// Alive reports whether the session with this id still signs in.
func Alive(id string) bool {
	auth := SignIn()
	return auth != nil && auth.Alive(context.Background(), id)
}

// clientIP finds the address of the client behind any proxy the config
// trusts. The web server sets it.
var clientIP = func(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// UseClientIP sets how the address of a client is found. The server calls it
// once before it starts serving.
func UseClientIP(f func(r *http.Request) string) { clientIP = f }

// ClientIP returns the address of the client behind r.
func ClientIP(r *http.Request) string { return clientIP(r) }

// ConfirmOwner makes sure the owner of the session is at the keyboard before
// the account gets a new lasting credential of another package, such as a key
// of the MCP server, so a stolen session cannot mint one.
func ConfirmOwner(r *http.Request) error {
	auth := SignIn()
	if auth == nil {
		return nil
	}
	err := auth.Mint(r, func() error { return nil })
	if errors.Is(err, signin.ErrNotFresh) {
		return signInAgain{auth}
	}
	return err
}

// signInAgain answers a change that wants a fresh sign-in as the sign-in
// does, so the page asks for the password or a new sign-in the same way
// for every change.
type signInAgain struct{ auth *signin.Auth }

func (signInAgain) Error() string { return "sign in again first" }

func (s signInAgain) Answer(w http.ResponseWriter, r *http.Request) { s.auth.SignInAgain(w, r) }

// Handler is the sign-in's handler with what CannonCommander adds to it.
// Its routes for admins also want an admin in the users table, so a row and
// an account that disagree never let anyone manage the sign-in. With
// passwords off in config.yaml nobody signs in with one or sets one.
func Handler(auth *signin.Auth) http.Handler {
	next := auth.Handler()
	prefix := settings.Config.Http.BaseURL + strings.TrimPrefix(signin.APIPrefix, "/")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route, under := strings.CutPrefix(r.URL.Path, prefix)
		switch {
		case !under:
		case adminRoute(route) && signedIn(r) && !usersAdmin(r):
			refuse(w, http.StatusForbidden, "forbidden")
			return
		case !settings.Config.Auth.Methods.PasswordAuth.Enabled && passwordRoute(r.Method, route):
			refuse(w, http.StatusForbidden, "passwordsOff")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func adminRoute(route string) bool {
	first, _, _ := strings.Cut(route, "/")
	switch first {
	case "accounts", "roles", "methods", "settings", "log":
		return true
	}
	return false
}

func passwordRoute(method, route string) bool {
	switch method + " " + route {
	case "PUT me/password", "POST login", "POST login/passkey/options", "POST login/passkey", "POST me/passkeys/options", "POST me/passkeys":
		return true
	}
	return false
}

func signedIn(r *http.Request) bool {
	_, ok := visitor(r)
	return ok
}

func usersAdmin(r *http.Request) bool {
	u, err := Authenticate(nil, r)
	return err == nil && u.Permissions.Admin
}

// refuse answers as the sign-in does, with a code.
func refuse(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
