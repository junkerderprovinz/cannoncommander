package accounts

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// basicMemory is how long a right password spares the next requests the hash
// check. Clients send it with every request, and a password hash is made to be
// slow.
const basicMemory = 5 * time.Minute

// basicBucket keeps the wrong passwords of WebDAV apart from the sign-in
// page, so a client with an old password does not lock its owner out of the
// browser.
const basicBucket = "dav"

// ErrBasicRefused says that a name and password do not sign in here. It does
// not tell why, as a password that is right but not enough stays a secret.
var ErrBasicRefused = errors.New("the name or the password is not right")

// TooManyTries says that a client failed too often and how long it has to wait.
type TooManyTries struct{ Wait time.Duration }

func (e *TooManyTries) Error() string {
	return "too many wrong tries, wait " + strconv.Itoa(int(e.Wait.Seconds())+1) + " seconds"
}

// Basic is who the credentials of a client that sends them with every
// request stand for: an API token in the password field, or a password that
// signs the account in on its own, without a second step.
type Basic struct {
	User     *users.User
	ReadOnly bool
	Token    bool
}

// BasicLogin checks the name and password of HTTP Basic. A token goes first:
// it cannot be guessed, so somebody guessing the password of the same
// account does not stop it.
func BasicLogin(r *http.Request, name, password string) (Basic, error) {
	auth := SignIn()
	if auth == nil || password == "" {
		return Basic{}, ErrBasicRefused
	}
	ctx := r.Context()
	if v, ok := auth.TokenVisitor(ctx, password); ok {
		u, err := userOf(v)
		if err != nil {
			return Basic{}, ErrBasicRefused
		}
		readOnly, err := TokenRights(u, v.Scopes)
		if err != nil {
			return Basic{}, ErrBasicRefused
		}
		return Basic{User: u, ReadOnly: readOnly, Token: true}, nil
	}
	if !settings.Config.Auth.Methods.PasswordAuth.Enabled {
		return Basic{}, ErrBasicRefused
	}
	name = strings.TrimSpace(name)
	if acc, ok := basicSeen.knows(r, auth, name, password); ok {
		u, err := userOf(signin.Visitor{Account: acc.ID, Role: acc.Role})
		if err == nil {
			return Basic{User: u}, nil
		}
	}
	v, err := auth.CheckPassword(r, basicBucket, name, password)
	var no *signin.Refusal
	if errors.As(err, &no) && no.Code == "throttled" {
		return Basic{}, &TooManyTries{Wait: time.Minute}
	}
	if err != nil {
		return Basic{}, ErrBasicRefused
	}
	acc, err := auth.Account(ctx, v.Account)
	if err != nil {
		return Basic{}, ErrBasicRefused
	}
	u, err := userOf(v)
	if err != nil {
		return Basic{}, ErrBasicRefused
	}
	basicSeen.remember(name, password, acc)
	return Basic{User: u}, nil
}

// passwordMemory remembers passwords that passed, as salted digests. Each one
// holds the account's epoch, so whatever ends its sessions ends it too, and
// every hit asks whether the account is blocked or has two-factor by now.
type passwordMemory struct {
	salt [32]byte

	mu    sync.Mutex
	known map[[32]byte]rememberedPassword
}

type rememberedPassword struct {
	account string
	epoch   int
	until   time.Time
}

var basicSeen = newPasswordMemory()

func newPasswordMemory() *passwordMemory {
	m := &passwordMemory{known: map[[32]byte]rememberedPassword{}}
	_, _ = rand.Read(m.salt[:])
	return m
}

func (m *passwordMemory) digest(name, password string) [32]byte {
	return sha256.Sum256([]byte(string(m.salt[:]) + name + "\x00" + password))
}

func (m *passwordMemory) knows(r *http.Request, auth *signin.Auth, name, password string) (signin.Account, bool) {
	m.mu.Lock()
	e, ok := m.known[m.digest(name, password)]
	m.mu.Unlock()
	if !ok || !time.Now().Before(e.until) {
		return signin.Account{}, false
	}
	acc, err := auth.Account(r.Context(), e.account)
	if err != nil || acc.Blocked || acc.TwoFactor || !acc.Password || acc.Epoch != e.epoch {
		return signin.Account{}, false
	}
	return acc, true
}

func (m *passwordMemory) remember(name, password string, acc signin.Account) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for k, e := range m.known {
		if now.After(e.until) {
			delete(m.known, k)
		}
	}
	m.known[m.digest(name, password)] = rememberedPassword{account: acc.ID, epoch: acc.Epoch, until: now.Add(basicMemory)}
}

// forget drops every password remembered for account.
func (m *passwordMemory) forget(account string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.known {
		if e.account == account {
			delete(m.known, k)
		}
	}
}

// userOf is the users row of the account v came in as.
func userOf(v signin.Visitor) (*users.User, error) {
	id, err := strconv.ParseUint(v.Account, 10, 64)
	if err != nil {
		return nil, err
	}
	u, err := state.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	if v.Role != roleAdmin {
		u.Permissions.Admin = false
	}
	return &u, nil
}

var errUnknownScope = errors.New("the token has a scope this server does not know")

// TokenRights narrows u to what an API token with scopes may do and reports
// whether the token only reads. Every way in that takes tokens goes through
// it, and it refuses a scope it does not know, so a scope added later never
// passes as full rights.
func TokenRights(u *users.User, scopes []string) (readOnly bool, err error) {
	caps := users.Permissions{Admin: true, Api: true, Share: true, Realtime: true}
	capped, all := false, false
	for _, s := range scopes {
		switch s {
		case scopeRead:
		case scopeAll:
			all = true
		case capNoAdmin:
			caps.Admin, capped = false, true
		case capNoAPI:
			caps.Api, capped = false, true
		case capNoShare:
			caps.Share, capped = false, true
		case capNoRealtime:
			caps.Realtime, capped = false, true
		default:
			return false, errUnknownScope
		}
	}
	if capped {
		u.Permissions = users.IntersectGlobalPermissions(u.Permissions, caps)
	}
	if !all {
		u.RestrictToReading()
	}
	return !all, nil
}
