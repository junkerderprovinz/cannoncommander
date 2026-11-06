package accounts

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// Me is the logged in user. Token is set when the request came with an
// API token.
type Me struct {
	Account
	Recovery     int        `json:"recovery"`
	LockPassword bool       `json:"lockPassword"`
	Token        *TokenInfo `json:"token,omitempty"`
}

// TokenInfo names the API token a request came with.
type TokenInfo struct {
	Name   string `json:"name"`
	Rights string `json:"rights"`
}

func (h handlers) me(r *http.Request, u *users.User) (Me, error) {
	p, err := h.profileOf(r.Context(), idOf(u))
	if err != nil {
		return Me{}, err
	}
	left, err := h.svc.recovery.left(r.Context(), u.ID)
	if err != nil {
		return Me{}, err
	}
	m := Me{Account: h.account(u, p), Recovery: left, LockPassword: u.LockPassword}
	if raw, ok := login.APIToken(r); ok {
		if name, found := users.TokenNameByRaw(u.Tokens, raw); found {
			m.Token = &TokenInfo{Name: name, Rights: rightsOf(u.Tokens[name])}
		}
	}
	return m, nil
}

func (h handlers) getMe(w http.ResponseWriter, r *http.Request) error {
	u, err := current(r)
	if err != nil {
		return err
	}
	m, err := h.me(r, &u)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, m)
}

type passwordChange struct {
	Current string `json:"current"`
	Next    string `json:"next"`
}

type passwordResult struct {
	// Ended counts the other sessions that were logged out.
	Ended int `json:"ended"`
}

// putPassword changes the own password and logs out every other session.
// API tokens keep working, they are revoked one by one.
func (h handlers) putPassword(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	var in passwordChange
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := current(r)
	if err != nil {
		return err
	}
	if u.LoginMethod != users.LoginMethodPassword {
		return v1.Coded(http.StatusConflict, "passwordElsewhere", "you sign in through "+string(u.LoginMethod)+", the password is kept there").With("method", string(u.LoginMethod))
	}
	if u.LockPassword {
		return v1.Coded(http.StatusForbidden, "passwordLocked", "your password is locked, an admin can change it")
	}
	if err := h.checkCurrentPassword(&u, in.Current); err != nil {
		return err
	}
	if err := checkPassword(in.Next); err != nil {
		return err
	}
	if in.Next == in.Current {
		return v1.Coded(http.StatusBadRequest, "samePassword", "the new password is the same as the old one")
	}
	u.RequirePasswordChange = false
	if err := state.UpdateUser(&u, in.Next, "password", "requirePasswordChange"); err != nil {
		return err
	}
	if err := passwordChanged(r.Context(), h.db, idOf(&u)); err != nil {
		return err
	}
	keep, _, _ := login.Session(r)
	ended, err := login.EndAll(u.ID, keep)
	if err != nil {
		return err
	}
	activity.RecordUserMutation(r, &activity.Actor{User: &u}, activitydb.EventUserUpdate, &u, []activitydb.FieldChange{{Field: "password"}})
	return v1.WriteJSON(w, r, http.StatusOK, passwordResult{Ended: ended})
}
