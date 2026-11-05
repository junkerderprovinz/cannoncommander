package accounts

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// basicMemory is how long a right password spares the next requests the hash
// check. Clients send it with every request, and a password hash is made to be
// slow.
const basicMemory = 5 * time.Minute

// ErrBasicRefused says that a name and password do not sign in here. It does
// not tell why, as a password that is right but not enough stays a secret.
var ErrBasicRefused = errors.New("the name or the password is not right")

// TooManyTries says that a client failed too often and how long it has to wait.
type TooManyTries struct{ Wait time.Duration }

func (e *TooManyTries) Error() string {
	return "too many wrong tries, wait " + strconv.Itoa(int(e.Wait.Seconds())+1) + " seconds"
}

// BasicLogin checks the name and password of a client that sends them with
// every request. Basic auth cannot carry a code, so it only lets in an account
// that a password alone would sign in. Failures count like at the sign in page
// but on their own, so a client with an old password does not lock its owner
// out of the browser.
func BasicLogin(db *sql.DB, r *http.Request, name, password string) (*users.User, error) {
	svc, err := serviceFor(db)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	ip := addressKey(r)
	pair := pairKey(name, login.ClientIP(r))
	if svc.basicTries.blocked(pair) {
		return nil, &TooManyTries{svc.basicTries.wait(pair)}
	}
	if svc.basicAddressTries.blocked(ip) {
		return nil, &TooManyTries{svc.basicAddressTries.wait(ip)}
	}
	u, err := state.GetUserByUsername(name)
	if err == nil && passwordIsEnough(&u) && svc.basicSeen.knows(name, password, u.Password) {
		return &u, nil
	}
	hash := utils.InvalidPasswordHash
	if err == nil {
		hash = u.Password
	}
	// The check runs for unknown names too, so the time taken tells nothing.
	if utils.CheckPwd(password, hash) != nil || err != nil || !passwordIsEnough(&u) {
		svc.basicTries.fail(pair)
		svc.basicAddressTries.fail(ip)
		return nil, ErrBasicRefused
	}
	svc.basicTries.clear(pair)
	svc.basicSeen.remember(name, password, u.Password)
	return &u, nil
}

// passwordIsEnough reports whether the sign in page would let u in on the
// password, without a further step.
func passwordIsEnough(u *users.User) bool {
	methods := settings.Config.Auth.Methods.PasswordAuth
	return methods.Enabled && !methods.EnforcedOtp && u.LoginMethod == users.LoginMethodPassword &&
		u.TOTPSecret == "" && !u.RequirePasswordChange
}

// passwordMemory remembers passwords that passed, as salted digests. Each one
// goes with the stored hash it passed against, so a new password ends it.
type passwordMemory struct {
	salt [32]byte

	mu    sync.Mutex
	known map[[32]byte]rememberedPassword
}

type rememberedPassword struct {
	hash  string
	until time.Time
}

func newPasswordMemory() *passwordMemory {
	m := &passwordMemory{known: map[[32]byte]rememberedPassword{}}
	_, _ = rand.Read(m.salt[:])
	return m
}

func (m *passwordMemory) digest(name, password string) [32]byte {
	return sha256.Sum256([]byte(string(m.salt[:]) + name + "\x00" + password))
}

func (m *passwordMemory) knows(name, password, hash string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.known[m.digest(name, password)]
	return ok && e.hash == hash && time.Now().Before(e.until)
}

func (m *passwordMemory) remember(name, password, hash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.known {
		if now.After(e.until) {
			delete(m.known, k)
		}
	}
	m.known[m.digest(name, password)] = rememberedPassword{hash, now.Add(basicMemory)}
}
