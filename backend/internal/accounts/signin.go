package accounts

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	ccerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/qrsvg"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/version"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// Steps of a sign in, as SignIn.Next names them.
const (
	stepDone      = "done"
	stepTotp      = "totp"
	stepPassword  = "password"
	stepTotpSetup = "totpSetup"
)

// stepTime is how long a sign in waits for its next step.
const stepTime = 5 * time.Minute

// SignIn says how a sign in goes on. With done the session cookie is set
// and Me is the account. Every other step comes with the challenge that
// carries the sign in to it.
type SignIn struct {
	Next      string     `json:"next"`
	Challenge string     `json:"challenge,omitempty"`
	Me        *Me        `json:"me,omitempty"`
	Setup     *TotpSetup `json:"setup,omitempty"`
	// Codes are the recovery codes of a two factor login that was set up
	// during the sign in, shown this once.
	Codes []string `json:"codes,omitempty"`
}

// SignInOptions tells the sign in page what this server offers.
type SignInOptions struct {
	Password bool   `json:"password"`
	Passkey  bool   `json:"passkey"`
	OIDC     *OIDC  `json:"oidc"`
	Server   string `json:"server"`
	Version  string `json:"version"`
	MinPass  int    `json:"minPassword"`
}

// OIDC is the single sign on of the older login, which the page links to.
type OIDC struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// PasskeyChallenge is what navigator.credentials.get needs, and the
// challenge the answer goes back with.
type PasskeyChallenge struct {
	Challenge string `json:"challenge"`
	Options   any    `json:"options"`
}

// pendingSignIn is a sign in that passed the password and waits for the
// step in need.
type pendingSignIn struct {
	userID uint64
	keep   bool
	need   string
	secret string
	until  time.Time
}

type pendingSignIns struct {
	mu   sync.Mutex
	list map[string]*pendingSignIn
}

func newPendingSignIns() *pendingSignIns {
	return &pendingSignIns{list: map[string]*pendingSignIn{}}
}

func (s *pendingSignIns) add(p *pendingSignIn) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, old := range s.list {
		if now.After(old.until) {
			delete(s.list, k)
		}
	}
	p.until = now.Add(stepTime)
	key := rand.Text()
	s.list[key] = p
	return key
}

// get returns the sign in behind challenge while it waits for step.
func (s *pendingSignIns) get(challenge, step string) (*pendingSignIn, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.list[challenge]
	if !ok || p.need != step || time.Now().After(p.until) {
		return nil, false
	}
	return p, true
}

func (s *pendingSignIns) drop(challenge string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.list, challenge)
}

var errStale = v1.Errorf(http.StatusGone, "this sign in ran out, please start again")

// signInOptions is public, the sign in page asks before anyone is known.
func (h handlers) signInOptions(w http.ResponseWriter, r *http.Request) error {
	methods := settings.Config.Auth.Methods
	out := SignInOptions{
		Password: methods.PasswordAuth.Enabled,
		Passkey:  auth.IsWebAuthnEnabled(),
		Server:   serverName(),
		Version:  version.Version,
		MinPass:  max(minPassword, settings.PasswordMinLength()),
	}
	if methods.OidcAuth.Enabled {
		label := settings.Config.Frontend.OIDCLoginButtonText
		if label == "" {
			label = "OpenID Connect"
		}
		out.OIDC = &OIDC{Label: label, URL: settings.Config.Http.BaseURL + "api/auth/oidc/login"}
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

// serverName is the name the sign in page shows under the app's name: the
// Unraid server's, or the host's elsewhere.
func serverName() string {
	if st, err := unraid.Default().State(); err == nil && st.Available && st.Name != "" {
		return st.Name
	}
	name, _ := os.Hostname()
	return name
}

type passwordSignIn struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// signIn checks name and password. Five wrong passwords for one name from
// one address block that pair for 15 minutes, and twenty from one address
// block the address, so nobody can try names one after the other.
func (h handlers) signIn(w http.ResponseWriter, r *http.Request) error {
	var in passwordSignIn
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if !settings.Config.Auth.Methods.PasswordAuth.Enabled {
		return v1.Errorf(http.StatusForbidden, "this server does not take passwords")
	}
	name := strings.TrimSpace(in.Username)
	if name == "" || in.Password == "" {
		return v1.Errorf(http.StatusBadRequest, "name and password are both needed")
	}
	ip := login.ClientIP(r)
	pair := strings.ToLower(name) + "|" + ip
	if h.svc.signInTries.blocked(pair) {
		return tooManyTries(w, h.svc.signInTries.wait(pair))
	}
	if h.svc.addressTries.blocked(ip) {
		return tooManyTries(w, h.svc.addressTries.wait(ip))
	}
	u, err := state.GetUserByUsername(name)
	hash := utils.InvalidPasswordHash
	if err == nil {
		hash = u.Password
	}
	// The check runs for unknown names too, so the time taken tells nothing.
	if utils.CheckPwd(in.Password, hash) != nil || err != nil || u.LoginMethod != users.LoginMethodPassword {
		h.svc.signInTries.fail(pair)
		h.svc.addressTries.fail(ip)
		return v1.Errorf(http.StatusUnauthorized, "the name or the password is not right")
	}
	h.svc.signInTries.clear(pair)
	p := &pendingSignIn{userID: u.ID, keep: in.Remember}
	return h.nextStep(w, r, &u, p, "")
}

// nextStep moves a sign in on to what it still needs, or finishes it.
// challenge is "" for a sign in that just passed the password.
func (h handlers) nextStep(w http.ResponseWriter, r *http.Request, u *users.User, p *pendingSignIn, challenge string) error {
	if challenge != "" {
		h.svc.signIns.drop(challenge)
	}
	switch {
	case u.RequirePasswordChange:
		p.need = stepPassword
	case u.TOTPSecret != "":
		p.need = stepTotp
	case settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp:
		key, err := auth.NewTotpKey(u.Username)
		if err != nil {
			return err
		}
		svg, err := qrsvg.Draw(key.URL())
		if err != nil {
			return err
		}
		p.need, p.secret = stepTotpSetup, key.Secret()
		next := h.svc.signIns.add(p)
		return v1.WriteJSON(w, r, http.StatusOK, SignIn{Next: stepTotpSetup, Challenge: next, Setup: &TotpSetup{Secret: key.Secret(), URI: key.URL(), QR: svg}})
	default:
		return h.finishSignIn(w, r, u, p.keep, nil)
	}
	return v1.WriteJSON(w, r, http.StatusOK, SignIn{Next: p.need, Challenge: h.svc.signIns.add(p)})
}

func (h handlers) finishSignIn(w http.ResponseWriter, r *http.Request, u *users.User, keep bool, codes []string) error {
	var stay time.Duration
	if keep {
		var err error
		if stay, err = prefs.StaySignedIn(r.Context(), h.db, u.ID); err != nil {
			return err
		}
	}
	if err := login.Start(w, r, u, stay); err != nil {
		return err
	}
	activity.RecordLogin(r, u)
	m, err := h.me(r, u)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, SignIn{Next: stepDone, Me: &m, Codes: codes})
}

type stepInput struct {
	Challenge string `json:"challenge"`
	Code      string `json:"code"`
	Password  string `json:"password"`
}

// pending returns the sign in behind challenge while it waits for step,
// with its user.
func (h handlers) pending(challenge, step string) (*pendingSignIn, *users.User, error) {
	p, ok := h.svc.signIns.get(challenge, step)
	if !ok {
		return nil, nil, errStale
	}
	u, err := state.GetUserByID(p.userID)
	if err != nil {
		h.svc.signIns.drop(challenge)
		return nil, nil, errStale
	}
	return p, &u, nil
}

// signInTotp takes a code from the authenticator app or a recovery code.
// Each works once: a code from the app counts for its time step only and
// a recovery code is used up. With the setup of a sign in that has to turn
// two factor login on, the code finishes the setup.
func (h handlers) signInTotp(w http.ResponseWriter, r *http.Request) error {
	var in stepInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if strings.TrimSpace(in.Code) == "" {
		return v1.Errorf(http.StatusBadRequest, "the code is missing")
	}
	if _, ok := h.svc.signIns.get(in.Challenge, stepTotpSetup); ok {
		return h.finishTotpSetup(w, r, in)
	}
	p, u, err := h.pending(in.Challenge, stepTotp)
	if err != nil {
		return err
	}
	if err := h.secondFactor(u, in.Code); err != nil {
		var pr *v1.Problem
		if errors.As(err, &pr) && pr.Status == http.StatusTooManyRequests {
			return tooManyTries(w, h.svc.factorTries.wait(idOf(u)))
		}
		return err
	}
	h.svc.signIns.drop(in.Challenge)
	return h.finishSignIn(w, r, u, p.keep, nil)
}

func (h handlers) finishTotpSetup(w http.ResponseWriter, r *http.Request, in stepInput) error {
	p, u, err := h.pending(in.Challenge, stepTotpSetup)
	if err != nil {
		return err
	}
	key := idOf(u)
	if h.svc.factorTries.blocked(key) {
		return tooManyTries(w, h.svc.factorTries.wait(key))
	}
	if !h.svc.totp.take(u.ID, p.secret, in.Code) {
		h.svc.factorTries.fail(key)
		return v1.Errorf(http.StatusBadRequest, "the code does not match. Is the clock on the phone right?")
	}
	h.svc.factorTries.clear(key)
	stored, nonce, err := auth.SealTotpSecret(p.secret)
	if err != nil {
		return err
	}
	u.TOTPSecret, u.TOTPNonce, u.OtpEnabled = stored, nonce, true
	if err := state.UpdateUser(u, "", "totpSecret", "totpNonce", "otpEnabled"); err != nil {
		return err
	}
	codes, err := h.svc.recovery.issue(r.Context(), u.ID)
	if err != nil {
		return err
	}
	h.svc.signIns.drop(in.Challenge)
	return h.finishSignIn(w, r, u, p.keep, codes)
}

// signInPassword sets the new password a sign in asks for, such as after
// the first start, when the admin password came from the log.
func (h handlers) signInPassword(w http.ResponseWriter, r *http.Request) error {
	var in stepInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	p, u, err := h.pending(in.Challenge, stepPassword)
	if err != nil {
		return err
	}
	if err := checkPassword(in.Password); err != nil {
		return err
	}
	if utils.CheckPwd(in.Password, u.Password) == nil {
		return v1.Errorf(http.StatusBadRequest, "the new password is the same as the old one")
	}
	u.RequirePasswordChange = false
	if err := state.UpdateUser(u, in.Password, "password", "requirePasswordChange"); err != nil {
		return err
	}
	if err := passwordChanged(r.Context(), h.db, idOf(u)); err != nil {
		return err
	}
	fresh, err := state.GetUserByID(u.ID)
	if err != nil {
		return err
	}
	return h.nextStep(w, r, &fresh, p, in.Challenge)
}

// passkeyOptions starts a sign in with a passkey. The browser picks the
// passkey, so the page needs no name.
func (h handlers) passkeyOptions(w http.ResponseWriter, r *http.Request) error {
	if !auth.IsWebAuthnEnabled() {
		return v1.Errorf(http.StatusConflict, "passkeys are turned off on this server")
	}
	ip := login.ClientIP(r)
	if h.svc.addressTries.blocked(ip) {
		return tooManyTries(w, h.svc.addressTries.wait(ip))
	}
	rpID, _ := login.PasskeySite(r)
	id, assertion, err := auth.GetWebAuthn().BeginPasskeyLogin(rpID)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, PasskeyChallenge{Challenge: id, Options: assertion})
}

type passkeySignIn struct {
	Challenge  string          `json:"challenge"`
	Credential json.RawMessage `json:"credential"`
	Remember   bool            `json:"remember"`
}

// signInPasskey finishes a passkey sign in. A passkey stands for both
// factors, so it skips the code from the app.
func (h handlers) signInPasskey(w http.ResponseWriter, r *http.Request) error {
	var in passkeySignIn
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	if !auth.IsWebAuthnEnabled() {
		return v1.Errorf(http.StatusConflict, "passkeys are turned off on this server")
	}
	ip := login.ClientIP(r)
	if h.svc.addressTries.blocked(ip) {
		return tooManyTries(w, h.svc.addressTries.wait(ip))
	}
	// The site has to be known before the answer is checked against it.
	login.PasskeySite(r)
	u, err := auth.GetWebAuthn().FinishPasskeyLogin(in.Challenge, in.Credential)
	if errors.Is(err, ccerrors.ErrPasskeyInvalidSession) {
		return errStale
	}
	if err != nil {
		h.svc.addressTries.fail(ip)
		return v1.Errorf(http.StatusUnauthorized, "this passkey does not sign in here")
	}
	h.svc.addressTries.clear(ip)
	if err := state.UpdateUser(u, "", "PasskeyCredentials"); err != nil {
		return err
	}
	return h.finishSignIn(w, r, u, in.Remember, nil)
}

// signOut ends the session the request came with.
func (h handlers) signOut(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	if err := login.Stop(w, r); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func tooManyTries(w http.ResponseWriter, wait time.Duration) error {
	w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
	return v1.Errorf(http.StatusTooManyRequests, "too many wrong tries, please wait a few minutes")
}
