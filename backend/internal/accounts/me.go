package accounts

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Me is the logged in user. Token is set when the request came with an
// API token.
type Me struct {
	Account
	// Home is the folder the file manager opens when the user picked none.
	Home         string     `json:"home"`
	Recovery     int        `json:"recovery"`
	LockPassword bool       `json:"lockPassword"`
	SignIn       bool       `json:"signIn"`
	HasPassword  bool       `json:"hasPassword"`
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
	m := Me{Account: h.account(u, p), Home: h.fs.Home(u), Recovery: left, LockPassword: u.LockPassword, SignIn: !state.OpenAccess(), HasPassword: u.Password != ""}
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

// putPassword sets or changes the own password and logs out every other
// session. API tokens keep working, they are revoked one by one.
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
	if err := ownPassword(&u); err != nil {
		return err
	}
	first := u.Password == ""
	if !first {
		if err := h.checkCurrentPassword(&u, in.Current); err != nil {
			return err
		}
	}
	if err := checkPassword(in.Next); err != nil {
		return err
	}
	if !first && in.Next == in.Current {
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
	ended, err := endSessions(r.Context(), h.db, &u, keep)
	if err != nil {
		return err
	}
	// The first password closes the server, and the browser that set it
	// stays in.
	if first {
		if err := login.Start(w, r, &u, 0); err != nil {
			return err
		}
	}
	activity.RecordUserMutation(r, &activity.Actor{User: &u}, activitydb.EventUserUpdate, &u, []activitydb.FieldChange{{Field: "password"}})
	return v1.WriteJSON(w, r, http.StatusOK, passwordResult{Ended: ended})
}

type passwordRemoval struct {
	Current string `json:"current"`
}

// deletePassword takes the password and everything else that signs the
// account in off it, which opens the server when nobody else signs in.
func (h handlers) deletePassword(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	var in passwordRemoval
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := current(r)
	if err != nil {
		return err
	}
	if err := ownPassword(&u); err != nil {
		return err
	}
	if err := h.checkCurrentPassword(&u, in.Current); err != nil {
		return err
	}
	all, err := state.GetAllUsers()
	if err != nil {
		return err
	}
	for i := range all {
		if all[i].ID != u.ID && state.SignsInHere(&all[i]) {
			return v1.Coded(http.StatusConflict, "othersSignIn", "other accounts still sign in here, so the password stays")
		}
	}
	if err := state.RemoveSignIn(u.ID); err != nil {
		return err
	}
	if err := h.svc.recovery.clear(r.Context(), u.ID); err != nil {
		return err
	}
	ended, err := endSessions(r.Context(), h.db, &u, "")
	if err != nil {
		return err
	}
	if err := login.Stop(w, r); err != nil {
		return err
	}
	activity.RecordUserMutation(r, &activity.Actor{User: &u}, activitydb.EventUserUpdate, &u, []activitydb.FieldChange{{Field: "password", To: "removed"}})
	return v1.WriteJSON(w, r, http.StatusOK, passwordResult{Ended: ended})
}

// ownPassword checks that u keeps a password on this server and may change
// it.
func ownPassword(u *users.User) error {
	if u.LoginMethod != users.LoginMethodPassword {
		return v1.Coded(http.StatusConflict, "passwordElsewhere", "you sign in through "+string(u.LoginMethod)+", the password is kept there").With("method", string(u.LoginMethod))
	}
	if u.LockPassword {
		return v1.Coded(http.StatusForbidden, "passwordLocked", "your password is locked, an admin can change it")
	}
	return nil
}

// passwordFirst refuses a credential for anyone while the server is open
// because nobody has one: the admin would lock themselves out.
func passwordFirst() error {
	if state.OpenAccess() && !settings.Config.Auth.Methods.NoAuth {
		return v1.Coded(http.StatusConflict, "ownPasswordFirst", "set your own password first")
	}
	return nil
}
