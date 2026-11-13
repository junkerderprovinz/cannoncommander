// Package accounts manages the own account and, for admins, users, groups,
// access rules and the activity log.
package accounts

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and its handlers.
func Routes(r *v1.Router) {
	r.Contract("accounts", contract)
	d := r.Deps()
	svc, err := serviceFor(d.DB)
	if err != nil {
		// Serving without the rules would show files they are meant to hide.
		panic(err)
	}
	s := sessionHandlers{db: d.DB}
	h := handlers{db: d.DB, fs: d.FS, deps: d, svc: svc}
	r.Handle("GET /me", h.getMe)
	r.Handle("PUT /me/password", h.putPassword)
	r.Handle("DELETE /me/password", h.deletePassword)
	r.Handle("GET /me/sessions", s.list)
	r.Handle("DELETE /me/sessions/{id}", s.end)
	r.Handle("GET /me/tokens", h.listTokens)
	r.Handle("POST /me/tokens", h.createToken)
	r.Handle("DELETE /me/tokens/{id}", h.deleteToken)
	r.Handle("GET /me/passkeys", h.listPasskeys)
	r.Handle("POST /me/passkeys", h.postPasskey)
	r.Handle("DELETE /me/passkeys/{id}", h.deletePasskey)
	r.Handle("POST /me/totp", h.postTotp)
	r.Handle("DELETE /me/totp", h.disableTotp)
	r.Handle("POST /auth/reset-code", h.askReset)
	r.Handle("POST /auth/reset", h.reset)
	r.Handle("GET /auth/options", h.signInOptions)
	r.Handle("POST /auth/login", h.signIn)
	r.Handle("POST /auth/totp", h.signInTotp)
	r.Handle("POST /auth/password", h.signInPassword)
	r.Handle("POST /auth/passkey/options", h.passkeyOptions)
	r.Handle("POST /auth/passkey", h.signInPasskey)
	r.Handle("POST /auth/logout", h.signOut)
	r.Handle("GET /admin/users", h.listUsers)
	r.Handle("POST /admin/users", h.createUser)
	r.Handle("GET /admin/users/{id}", h.getUser)
	r.Handle("PATCH /admin/users/{id}", h.updateUser)
	r.Handle("DELETE /admin/users/{id}", h.deleteUser)
	r.Handle("GET /admin/users/{id}/tokens", h.userTokens)
	r.Handle("DELETE /admin/users/{id}/tokens/{token}", h.revokeUserToken)
	r.Handle("GET /admin/users/{id}/passkeys", h.userPasskeys)
	r.Handle("DELETE /admin/users/{id}/passkeys/{passkey}", h.removeUserPasskey)
	r.Handle("GET /admin/users/{id}/sessions", h.userSessions)
	r.Handle("DELETE /admin/users/{id}/sessions/{session}", h.endUserSession)
	r.Handle("GET /admin/groups", h.listGroups)
	r.Handle("POST /admin/groups", h.createGroup)
	r.Handle("PATCH /admin/groups/{name}", h.updateGroup)
	r.Handle("DELETE /admin/groups/{name}", h.deleteGroup)
	r.Handle("GET /admin/rules", h.getRules)
	r.Handle("PUT /admin/rules", h.putRules)
	r.Handle("POST /admin/rules/test", h.testRules)
	r.Handle("GET /admin/activity", h.listActivity)
}
