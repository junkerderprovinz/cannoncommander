package accounts

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

const (
	resetCodeLen = 12
	resetTime    = 10 * time.Minute
)

// announce writes a reset code to the container log, which only the admin
// of the server can read. Tests catch it here.
var announce = func(username, code string) {
	logger.Warningf("Someone asked to reset the password of %s. The code is %s and works for 10 minutes.", username, code)
}

// resets keeps the codes that were written to the log, one per user.
type resets struct {
	mu    sync.Mutex
	codes map[string]resetCode
}

type resetCode struct {
	userID uint64
	until  time.Time
}

func newResets() *resets {
	return &resets{codes: map[string]resetCode{}}
}

// issue makes a code for the user. It returns "" while the last one is still
// good: anyone can ask, and a new code would take the old one from the owner.
func (s *resets) issue(userID uint64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	live := false
	for code, c := range s.codes {
		switch {
		case now.After(c.until):
			delete(s.codes, code)
		case c.userID == userID:
			live = true
		}
	}
	if live {
		return ""
	}
	code := randomCode(resetCodeLen)
	s.codes[code] = resetCode{userID: userID, until: now.Add(resetTime)}
	return code
}

// redeem uses up a code and returns the user it was made for.
func (s *resets) redeem(code string) (uint64, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.codes[code]
	delete(s.codes, code)
	if !ok || time.Now().After(c.until) {
		return 0, false
	}
	return c.userID, true
}

type resetRequest struct {
	Username string `json:"username"`
}

// askReset writes a reset code to the log. The answer is the same whether
// the name exists or not, so nobody can probe for accounts this way.
func (h handlers) askReset(w http.ResponseWriter, r *http.Request) error {
	var in resetRequest
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := state.GetUserByUsername(strings.TrimSpace(in.Username))
	if err == nil && u.LoginMethod == users.LoginMethodPassword {
		if code := h.svc.resets.issue(u.ID); code != "" {
			announce(u.Username, code[:4]+"-"+code[4:8]+"-"+code[8:])
		}
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

type resetInput struct {
	Code     string `json:"code"`
	Password string `json:"password"`
}

// reset sets a new password with a code from the log. It also ends every
// session, turns off the two factor login and removes every passkey and API
// token, because whoever lost the password may have lost more. The client is
// logged in afterwards.
func (h handlers) reset(w http.ResponseWriter, r *http.Request) error {
	var in resetInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	ip := addressKey(r)
	if h.svc.resetTries.blocked(ip) {
		return v1.Coded(http.StatusTooManyRequests, "tooManyTries", "too many wrong codes, wait a few minutes")
	}
	code := normalizeCode(in.Code, resetCodeLen)
	if code == "" {
		return v1.Coded(http.StatusBadRequest, "resetCodeLength", "the code has twelve characters, just as it is in the log")
	}
	if err := checkPassword(in.Password); err != nil {
		return err
	}
	id, ok := h.svc.resets.redeem(code)
	if !ok {
		h.svc.resetTries.fail(ip)
		return v1.Coded(http.StatusBadRequest, "resetCodeWrong", "this code is not right or ran out, please ask for a new one")
	}
	h.svc.resetTries.clear(ip)
	u, err := state.GetUserByID(id)
	if err != nil {
		return v1.Coded(http.StatusBadRequest, "resetCodeWrong", "this code is not right or ran out, please ask for a new one")
	}
	u.OtpEnabled, u.RequirePasswordChange, u.PasskeyCredentials = false, false, nil
	if err := state.UpdateUser(&u, in.Password, "password", "otpEnabled", "requirePasswordChange", "PasskeyCredentials"); err != nil {
		return err
	}
	if err := h.svc.recovery.clear(r.Context(), u.ID); err != nil {
		return err
	}
	if err := h.revokeTokens(r, &u); err != nil {
		return err
	}
	if _, err := login.EndAll(u.ID, ""); err != nil {
		return err
	}
	if err := passwordChanged(r.Context(), h.db, idOf(&u)); err != nil {
		return err
	}
	if u, err = state.GetUserByID(id); err != nil {
		return err
	}
	if err := login.Start(w, r, &u, 0); err != nil {
		return err
	}
	activity.RecordAuth(r, &u, activitydb.EventUserUpdate, activitydb.Details{Changes: []activitydb.FieldChange{{Field: "password", To: "reset"}}})
	m, err := h.me(r, &u)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, m)
}
