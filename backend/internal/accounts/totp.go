package accounts

import (
	"net/http"
	"strings"
	"sync"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// setupTime is how long a started setup waits for the first code.
const setupTime = 10 * time.Minute

// totpSetups holds the secrets of setups that wait for their first code,
// and the last time step each user logged a code in with, so a code that
// was seen once is never taken again.
type totpSetups struct {
	mu      sync.Mutex
	pending map[uint64]pendingSetup
	used    map[uint64]int64
}

type pendingSetup struct {
	secret string
	until  time.Time
}

func newTotpSetups() *totpSetups {
	return &totpSetups{pending: map[uint64]pendingSetup{}, used: map[uint64]int64{}}
}

func (s *totpSetups) start(userID uint64, secret string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for id, p := range s.pending {
		if now.After(p.until) {
			delete(s.pending, id)
		}
	}
	s.pending[userID] = pendingSetup{secret: secret, until: now.Add(setupTime)}
}

func (s *totpSetups) waiting(userID uint64) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.pending[userID]
	if !ok || time.Now().After(p.until) {
		return "", false
	}
	return p.secret, true
}

func (s *totpSetups) done(userID uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, userID)
}

// take accepts code for secret once per time step.
func (s *totpSetups) take(userID uint64, secret, code string) bool {
	step, ok := auth.TotpStep(secret, strings.TrimSpace(code), time.Now())
	if !ok {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if last, seen := s.used[userID]; seen && step <= last {
		return false
	}
	s.used[userID] = step
	return true
}

// TotpSetup is the secret for the authenticator app, as text, as the
// otpauth URI and as that URI drawn as a QR code in SVG.
type TotpSetup struct {
	Secret string `json:"secret"`
	URI    string `json:"uri"`
	QR     string `json:"qr"`
}

// totpInput drives the three steps behind POST /me/totp: nothing starts
// the setup, a code finishes it, and a code with regenerate makes new
// recovery codes.
type totpInput struct {
	Code       string `json:"code"`
	Regenerate bool   `json:"regenerate"`
}

type codeInput struct {
	Code string `json:"code"`
}

// RecoveryList holds fresh recovery codes, shown this once.
type RecoveryList struct {
	Codes []string `json:"codes"`
}

// totpUser loads the logged in user for a change to the second factor.
func totpUser(r *http.Request) (users.User, error) {
	if err := needSession(r); err != nil {
		return users.User{}, err
	}
	u, err := current(r)
	if err != nil {
		return users.User{}, err
	}
	if u.LoginMethod != users.LoginMethodPassword {
		return users.User{}, v1.Errorf(http.StatusConflict, "two factor login comes with the password, and you sign in through "+string(u.LoginMethod))
	}
	return u, nil
}

func (h handlers) postTotp(w http.ResponseWriter, r *http.Request) error {
	var in totpInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := totpUser(r)
	if err != nil {
		return err
	}
	switch {
	case in.Regenerate:
		return h.renewRecovery(w, r, &u, in.Code)
	case in.Code != "":
		return h.confirmTotp(w, r, &u, in.Code)
	}
	return h.startTotp(w, r, &u)
}

func (h handlers) startTotp(w http.ResponseWriter, r *http.Request, u *users.User) error {
	if u.TOTPSecret != "" {
		return v1.Errorf(http.StatusConflict, "two factor login is on already")
	}
	key, err := auth.NewTotpKey(u.Username)
	if err != nil {
		return err
	}
	svg, err := qrSVG(key.URL())
	if err != nil {
		return err
	}
	h.svc.totp.start(u.ID, key.Secret())
	return v1.WriteJSON(w, r, http.StatusOK, TotpSetup{Secret: key.Secret(), URI: key.URL(), QR: svg})
}

func (h handlers) confirmTotp(w http.ResponseWriter, r *http.Request, u *users.User, code string) error {
	if u.TOTPSecret != "" {
		return v1.Errorf(http.StatusConflict, "two factor login is on already")
	}
	secret, ok := h.svc.totp.waiting(u.ID)
	if !ok {
		return v1.Errorf(http.StatusConflict, "the setup ran out, please start it again")
	}
	key := idOf(u)
	if h.svc.factorTries.blocked(key) {
		return v1.Errorf(http.StatusTooManyRequests, "too many wrong codes, wait a few minutes")
	}
	if !h.svc.totp.take(u.ID, secret, code) {
		h.svc.factorTries.fail(key)
		return v1.Errorf(http.StatusBadRequest, "the code does not match. Is the clock on the phone right?")
	}
	h.svc.factorTries.clear(key)
	stored, nonce, err := auth.SealTotpSecret(secret)
	if err != nil {
		return err
	}
	u.TOTPSecret, u.TOTPNonce, u.OtpEnabled = stored, nonce, true
	if err := state.UpdateUser(u, "", "totpSecret", "totpNonce", "otpEnabled"); err != nil {
		return err
	}
	h.svc.totp.done(u.ID)
	codes, err := h.svc.recovery.issue(r.Context(), u.ID)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, RecoveryList{Codes: codes})
}

func (h handlers) renewRecovery(w http.ResponseWriter, r *http.Request, u *users.User, code string) error {
	if err := h.secondFactor(u, code); err != nil {
		return err
	}
	codes, err := h.svc.recovery.issue(r.Context(), u.ID)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, RecoveryList{Codes: codes})
}

// secondFactor checks a code from the app or a recovery code of u. A
// recovery code is used up by it.
func (h handlers) secondFactor(u *users.User, code string) error {
	if u.TOTPSecret == "" {
		return v1.Errorf(http.StatusConflict, "two factor login is off")
	}
	key := idOf(u)
	if h.svc.factorTries.blocked(key) {
		return v1.Errorf(http.StatusTooManyRequests, "too many wrong codes, wait a few minutes")
	}
	secret, err := auth.OpenTotpSecret(u.TOTPSecret, u.TOTPNonce)
	if err != nil {
		return err
	}
	if !h.svc.totp.take(u.ID, secret, code) && !h.svc.recovery.Redeem(u.ID, code) {
		h.svc.factorTries.fail(key)
		return v1.Errorf(http.StatusBadRequest, "this code does not work, take a fresh one from the app or a recovery code")
	}
	h.svc.factorTries.clear(key)
	return nil
}

func (h handlers) disableTotp(w http.ResponseWriter, r *http.Request) error {
	var in codeInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := totpUser(r)
	if err != nil {
		return err
	}
	if settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp {
		return v1.Errorf(http.StatusConflict, "this server asks everyone for two factor login")
	}
	if err := h.secondFactor(&u, in.Code); err != nil {
		return err
	}
	u.OtpEnabled = false
	if err := state.UpdateUser(&u, "", "otpEnabled"); err != nil {
		return err
	}
	if err := h.svc.recovery.clear(r.Context(), u.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
