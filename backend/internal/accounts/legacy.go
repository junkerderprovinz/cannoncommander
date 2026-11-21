package accounts

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	cerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// OldCookie is the session cookie of the version before.
const OldCookie = "cannoncommander_session"

// codeAlphabet leaves out the letters and digits that look alike. Recovery
// codes of the version before are 8 of them.
const codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func (h handlers) legacy() signin.Legacy {
	return signin.Legacy{
		Password: func(_ context.Context, hash, pw string) (bool, error) { return utils.CheckPwd(pw, hash) == nil, nil },
		Session:  OldSession,
		Recovery: h.oldRecovery,
		Token:    oldToken,
		Drop:     h.drop,
	}
}

type oldClaims struct {
	jwt.RegisteredClaims
	SignedIn *jwt.NumericDate `json:"auth_time,omitempty"`
}

// OldSession reads the session cookie of the version before. A browser can
// carry two of that name, one for the host and one for a parent domain, so
// every one counts, but two of different accounts swap neither: one of them
// was planted.
func OldSession(r *http.Request) (signin.OldSession, bool) {
	var found signin.OldSession
	for _, c := range r.Cookies() {
		if c.Name != OldCookie {
			continue
		}
		s, ok := readOldSession(c.Value)
		if !ok {
			continue
		}
		if found.Account != "" && found.Account != s.Account {
			return signin.OldSession{}, false
		}
		if found.Account == "" {
			found = s
		}
	}
	return found, found.Account != ""
}

func readOldSession(raw string) (signin.OldSession, bool) {
	if strings.Count(raw, ".") != 2 || len(raw) > 4096 {
		return signin.OldSession{}, false
	}
	var claims oldClaims
	if _, err := auth.ParseToken(raw, &claims); err != nil || claims.IssuedAt == nil || claims.ExpiresAt == nil {
		return signin.OldSession{}, false
	}
	owner, isSession, ok := state.HashedTokenOwner(raw)
	if !ok || !isSession || state.IsTokenRevoked(raw) {
		return signin.OldSession{}, false
	}
	u, err := state.GetUserByID(owner)
	if err != nil {
		return signin.OldSession{}, false
	}
	s := signin.OldSession{
		Account: idOf(&u), Key: utils.HashSHA256(raw), Issued: claims.IssuedAt.Time,
		SignedAt: claims.IssuedAt.Time, Expires: claims.ExpiresAt.Time, Via: signin.ViaPassword,
	}
	if claims.SignedIn != nil {
		s.SignedAt = claims.SignedIn.Time
	}
	if u.LoginMethod == users.LoginMethodOidc && settings.Config.Auth.Methods.OidcAuth.Enabled {
		s.Via, s.Service = signin.ViaService, configServiceID
	}
	return s, true
}

// oldToken returns the digest of an API token of the version before. It
// does not look the token up: the sign-in has it or not, and Drop clears
// the old registry without ending what the sign-in still holds.
func oldToken(raw string) (string, bool) {
	if strings.Count(raw, ".") != 2 || len(raw) > 4096 {
		return "", false
	}
	if _, err := auth.ParseToken(raw, &users.AuthToken{}); err != nil {
		return "", false
	}
	return utils.HashSHA256(raw), true
}

// oldRecovery checks a recovery code of the version before, and marks the
// code used in its table too, so going back cannot use it once more.
func (h handlers) oldRecovery(hash, code string) bool {
	c := strings.ToUpper(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code)))
	if len(c) != 8 || strings.Trim(c, codeAlphabet) != "" || utils.CheckPwd(c, hash) != nil {
		return false
	}
	_ = state.InLegacyTx(func(tx *sql.Tx) error {
		_, err := tx.Exec(`UPDATE recovery_codes SET used_at = ? WHERE code_hash = ? AND used_at = 0`, time.Now().Unix(), hash)
		return err
	})
	return true
}

// drop makes the version before unable to let account in on what the
// sign-in ended. After another account's reset only sessions and tokens go,
// since the password still stands.
func (h handlers) drop(ctx context.Context, account string, what signin.Ending) error {
	switch what {
	case signin.EndResetOther:
		return shutOut(ctx, account, oldSessions)
	case signin.EndBlocked, signin.EndRemoved:
		return shutOut(ctx, account, everyOldWay)
	}
	return shutOut(ctx, account, oldWaysButOIDC)
}

// How far shutOut goes.
type oldWays int

const (
	oldSessions    oldWays = iota // the old sessions and tokens
	oldWaysButOIDC                // also password, second factor, passkeys and recovery codes
	everyOldWay                   // also the OIDC binding the version before signs in by
)

// shutOut takes ways into account away from the version before. Every
// token and session is revoked in its registry before the token list goes,
// so no step leaves a token with more rights, and the password becomes one
// nobody knows, never an empty one, which would open that version.
func shutOut(ctx context.Context, account string, how oldWays) error {
	id, err := strconv.ParseUint(account, 10, 64)
	if err != nil {
		return nil
	}
	u, err := state.GetUserByID(id)
	if errors.Is(err, cerrors.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, t := range u.Tokens {
		if t.Token == "" {
			continue
		}
		if err := state.RevokeTokenHash(utils.HashSHA256(t.Token)); err != nil {
			return err
		}
	}
	if _, err := state.RevokeSessions(id, ""); err != nil {
		return err
	}
	if err := state.ForgetTokenOwner(id); err != nil {
		return err
	}
	u.Tokens = nil
	fields := []string{"tokens"}
	password := ""
	gone := []string{"sessions", "token_use"}
	if how >= oldWaysButOIDC {
		password = unusablePassword()
		u.OtpEnabled, u.PasskeyCredentials = false, nil
		fields = append(fields, "password", "otpEnabled", "PasskeyCredentials")
		gone = append(gone, "recovery_codes")
	}
	// The version before binds an OIDC account by its name, so an account
	// that is blocked or goes becomes one that signs in with a password.
	if how == everyOldWay {
		u.LoginMethod = users.LoginMethodPassword
		fields = append(fields, "loginMethod")
		gone = append(gone, "oidc_identities")
	}
	if err := state.UpdateUser(&u, password, fields...); err != nil {
		return err
	}
	return state.InLegacyTx(func(tx *sql.Tx) error {
		for _, table := range gone {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE user_id = ?`, account); err != nil {
				return err
			}
		}
		return nil
	})
}

// unusablePassword is a password nobody knows, for the version before to
// hash, so its account counts as one that signs in.
func unusablePassword() string { return rand.Text() + rand.Text() }

// mirrorPassword gives the row of an account that now has a password in the
// sign-in one nobody knows, unless it has one: the version before then
// counts it as one that signs in and stays closed, and cannot bind it to an
// OIDC identity by its name.
func mirrorPassword(ctx context.Context, account string) error {
	id, err := strconv.ParseUint(account, 10, 64)
	if err != nil {
		return nil
	}
	u, err := state.GetUserByID(id)
	if errors.Is(err, cerrors.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if u.LoginMethod == users.LoginMethodPassword && u.Password != "" {
		return nil
	}
	u.LoginMethod = users.LoginMethodPassword
	if err := state.UpdateUser(&u, unusablePassword(), "password", "loginMethod"); err != nil {
		return err
	}
	return state.InLegacyTx(func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM oidc_identities WHERE user_id = ?`, account)
		return err
	})
}
