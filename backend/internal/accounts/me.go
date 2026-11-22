package accounts

import (
	"errors"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// Me is the signed in user as CannonCommander sees it. The sign-in's own
// view of the account is GET /api/signin/v1/me. Token is set when the
// request came with an API token.
type Me struct {
	Account
	// Home is the folder the file manager opens when the user picked none.
	Home        string     `json:"home"`
	SignIn      bool       `json:"signIn"`
	HasPassword bool       `json:"hasPassword"`
	Token       *TokenInfo `json:"token,omitempty"`
}

// TokenInfo tells what the API token a request came with may do.
type TokenInfo struct {
	Rights string `json:"rights"`
}

func (h handlers) getMe(w http.ResponseWriter, r *http.Request) error {
	u, err := current(r)
	if err != nil {
		return err
	}
	p, err := h.profileOf(r.Context(), idOf(&u))
	if err != nil {
		return err
	}
	var acc signin.Account
	if auth := SignIn(); auth != nil {
		acc, err = auth.Account(r.Context(), idOf(&u))
		if err != nil && !errors.Is(err, signin.ErrNotFound) {
			return err
		}
	}
	m := Me{Account: h.account(&u, p, acc), Home: h.fs.Home(&u), SignIn: !state.OpenAccess(), HasPassword: acc.Password}
	if v, ok := visitor(r); ok && v.Via == signin.ViaToken {
		m.Token = &TokenInfo{Rights: rightsOf(v.Scopes)}
	}
	return v1.WriteJSON(w, r, http.StatusOK, m)
}
