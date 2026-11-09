package accounts

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// The handlers here let admins see and take away the ways into another
// account. Taking away needs a session, like the user's own routes, so an
// admin token cannot lock people out.

func (h handlers) userTokens(w http.ResponseWriter, r *http.Request) error {
	u, err := target(r)
	if err != nil {
		return err
	}
	return h.writeTokens(w, r, &u)
}

func (h handlers) revokeUserToken(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	u, err := target(r)
	if err != nil {
		return err
	}
	tok, err := tokenByID(&u, r.PathValue("token"))
	if err != nil {
		return err
	}
	if err := h.revokeToken(r, &u, tok); err != nil {
		return err
	}
	recordForUser(r, activitydb.EventTokenDelete, activitydb.Details{TargetUsername: u.Username, AffectedTokenName: tok.Name})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) userPasskeys(w http.ResponseWriter, r *http.Request) error {
	u, err := target(r)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Passkey{"items": passkeysOf(&u)})
}

func (h handlers) removeUserPasskey(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	u, err := target(r)
	if err != nil {
		return err
	}
	name, err := removePasskey(&u, r.PathValue("passkey"))
	if err != nil {
		return err
	}
	recordForUser(r, activitydb.EventPasskeyDelete, activitydb.Details{TargetUsername: u.Username, PasskeyName: name})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) userSessions(w http.ResponseWriter, r *http.Request) error {
	u, err := target(r)
	if err != nil {
		return err
	}
	return sessionHandlers{db: h.db}.write(w, r, u.ID)
}

func (h handlers) endUserSession(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	u, err := target(r)
	if err != nil {
		return err
	}
	return sessionHandlers{db: h.db}.endOne(w, r, u.ID, r.PathValue("session"))
}

// revokeAll ends every session, API token and passkey of u.
func (h handlers) revokeAll(r *http.Request, u *users.User) error {
	if _, err := endSessions(r.Context(), h.db, u, ""); err != nil {
		return err
	}
	if err := h.revokeTokens(r, u); err != nil {
		return err
	}
	if len(u.PasskeyCredentials) == 0 {
		return nil
	}
	u.PasskeyCredentials = nil
	return state.UpdateUser(u, "", "PasskeyCredentials")
}

func recordForUser(r *http.Request, event activitydb.EventType, details activitydb.Details) {
	activity.RecordUser(r, &activity.Actor{User: v1.UserFrom(r)}, activitydb.Entry{EventType: event, Details: details})
}
