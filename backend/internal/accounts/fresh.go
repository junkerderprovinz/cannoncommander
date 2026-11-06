package accounts

import (
	"net/http"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// freshFor is how long a sign in counts as fresh for an account that has no
// password to type.
const freshFor = 5 * time.Minute

// errStaleSignIn tells the page to sign in again before it retries.
var errStaleSignIn = &v1.Problem{
	Type:   "fresh-sign-in",
	Title:  http.StatusText(http.StatusForbidden),
	Status: http.StatusForbidden,
	Detail: "sign in again, then try once more within five minutes",
}

// confirmOwner makes sure the owner of the session is at the keyboard before
// the account gets a new lasting credential, so a stolen session cannot mint
// one. Accounts with a password type it, the others need a fresh sign in.
func (h handlers) confirmOwner(r *http.Request, u *users.User, password string) error {
	if u.LoginMethod == users.LoginMethodPassword {
		return h.checkCurrentPassword(u, password)
	}
	at, ok := login.SignedInAt(r)
	if !ok || time.Since(at) > freshFor {
		return errStaleSignIn
	}
	return nil
}

// checkCurrentPassword checks the password u signs in with. Wrong ones count
// against the account, five of them block the check for 15 minutes.
func (h handlers) checkCurrentPassword(u *users.User, password string) error {
	key := idOf(u)
	if h.svc.passwordTries.blocked(key) {
		return v1.Errorf(http.StatusTooManyRequests, "too many wrong passwords, wait a few minutes")
	}
	if password == "" {
		return v1.Errorf(http.StatusBadRequest, "the current password is missing")
	}
	if utils.CheckPwd(password, u.Password) != nil {
		h.svc.passwordTries.fail(key)
		return v1.Errorf(http.StatusForbidden, "the current password is not right")
	}
	h.svc.passwordTries.clear(key)
	return nil
}
