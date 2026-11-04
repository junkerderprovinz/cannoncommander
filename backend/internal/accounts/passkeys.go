package accounts

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	ccerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// Passkey is one registered passkey. Origin is the host it works for.
type Passkey struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Origin  string     `json:"origin"`
	Created time.Time  `json:"created"`
	Last    *time.Time `json:"last"`
}

// PasskeyList says whether the server takes passkeys at all.
type PasskeyList struct {
	Enabled bool      `json:"enabled"`
	Items   []Passkey `json:"items"`
}

func passkeyOf(c users.WebAuthnCredential) Passkey {
	return Passkey{ID: c.ID, Name: c.Name, Origin: c.Origin, Created: time.Unix(c.CreatedAt, 0).UTC(), Last: unixTime(c.LastUsedAt)}
}

func (h handlers) listPasskeys(w http.ResponseWriter, r *http.Request) error {
	u, err := current(r)
	if err != nil {
		return err
	}
	items := []Passkey{}
	for _, c := range u.PasskeyCredentials {
		items = append(items, passkeyOf(c))
	}
	return v1.WriteJSON(w, r, http.StatusOK, PasskeyList{Enabled: auth.IsWebAuthnEnabled(), Items: items})
}

// passkeyUser loads the user for a change to the passkeys and checks the
// name a new one is to get.
func passkeyUser(r *http.Request, name string) (users.User, error) {
	if err := needSession(r); err != nil {
		return users.User{}, err
	}
	if !auth.IsWebAuthnEnabled() {
		return users.User{}, v1.Errorf(http.StatusConflict, "passkeys are turned off on this server")
	}
	u, err := current(r)
	if err != nil {
		return users.User{}, err
	}
	switch {
	case name == "":
		return users.User{}, v1.Errorf(http.StatusBadRequest, "give the passkey a name")
	case len(name) > 64:
		return users.User{}, v1.Errorf(http.StatusBadRequest, "the name can have at most 64 characters")
	case slices.ContainsFunc(u.PasskeyCredentials, func(c users.WebAuthnCredential) bool { return c.Name == name }):
		return users.User{}, v1.Errorf(http.StatusConflict, "there is a passkey with this name already")
	}
	return u, nil
}

// passkeyInput drives both steps behind POST /me/passkeys: a name alone
// starts a registration, the name with the browser's credential ends it.
type passkeyInput struct {
	Name       string          `json:"name"`
	Credential json.RawMessage `json:"credential"`
}

// PasskeyStep is the answer to either step. Options go to
// navigator.credentials.create, Passkey is the one that was added.
type PasskeyStep struct {
	Options any      `json:"options,omitempty"`
	Passkey *Passkey `json:"passkey,omitempty"`
}

func (h handlers) postPasskey(w http.ResponseWriter, r *http.Request) error {
	var in passkeyInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	u, err := passkeyUser(r, name)
	if err != nil {
		return err
	}
	rpID, host := login.PasskeySite(r)
	if len(in.Credential) == 0 || string(in.Credential) == "null" {
		session, creation, err := auth.GetWebAuthn().BeginRegistration(&u, rpID)
		if err != nil {
			return err
		}
		h.svc.passkeys.start(u.ID, session, name)
		return v1.WriteJSON(w, r, http.StatusOK, PasskeyStep{Options: creation})
	}
	session, ok := h.svc.passkeys.take(u.ID, name)
	if !ok {
		return v1.Errorf(http.StatusConflict, "the registration ran out, please start it again")
	}
	if err := auth.GetWebAuthn().FinishRegistrationJSON(&u, session, name, in.Credential); err != nil {
		if errors.Is(err, ccerrors.ErrPasskeyExists) {
			return v1.Errorf(http.StatusConflict, "this passkey is registered already")
		}
		return v1.Errorf(http.StatusBadRequest, "the browser did not confirm the passkey, please try again")
	}
	added := &u.PasskeyCredentials[len(u.PasskeyCredentials)-1]
	added.Origin, added.LastUsedAt = host, 0
	if err := state.UpdateUser(&u, "", "PasskeyCredentials"); err != nil {
		return err
	}
	activity.RecordAuth(r, &u, activitydb.EventPasskeyRegister, activitydb.Details{PasskeyName: name})
	p := passkeyOf(*added)
	return v1.WriteJSON(w, r, http.StatusCreated, PasskeyStep{Passkey: &p})
}

// passkeyStarts remembers the registration each user started, so the
// second step needs nothing but the name and the credential.
type passkeyStarts struct {
	mu      sync.Mutex
	pending map[uint64]passkeyStart
}

type passkeyStart struct {
	session, name string
	until         time.Time
}

func newPasskeyStarts() *passkeyStarts {
	return &passkeyStarts{pending: map[uint64]passkeyStart{}}
}

func (s *passkeyStarts) start(userID uint64, session, name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[userID] = passkeyStart{session: session, name: name, until: time.Now().Add(5 * time.Minute)}
}

// take hands out the started registration once, if it was for this name.
func (s *passkeyStarts) take(userID uint64, name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[userID]
	delete(s.pending, userID)
	if !ok || p.name != name || time.Now().After(p.until) {
		return "", false
	}
	return p.session, true
}

func (h handlers) deletePasskey(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	u, err := current(r)
	if err != nil {
		return err
	}
	i := slices.IndexFunc(u.PasskeyCredentials, func(c users.WebAuthnCredential) bool { return c.ID == r.PathValue("id") })
	if i < 0 {
		return v1.Errorf(http.StatusNotFound, "there is no passkey with this id")
	}
	name := u.PasskeyCredentials[i].Name
	u.PasskeyCredentials = slices.Delete(u.PasskeyCredentials, i, i+1)
	if err := state.UpdateUser(&u, "", "PasskeyCredentials"); err != nil {
		return err
	}
	activity.RecordAuth(r, &u, activitydb.EventPasskeyDelete, activitydb.Details{PasskeyName: name})
	w.WriteHeader(http.StatusNoContent)
	return nil
}
