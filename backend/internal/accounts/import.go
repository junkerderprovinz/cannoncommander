package accounts

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/golang-jwt/jwt/v5"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// neverExpires is how far off an API token that never runs out had its
// expiry, as the version before wrote one anyway.
const neverExpires = 50 * 365 * 24 * time.Hour

// Legacy capped tokens keep their caps as scopes that take a right away.
const (
	scopeRead     = "read"
	scopeAll      = "all"
	capNoAdmin    = "-admin"
	capNoAPI      = "-api"
	capNoShare    = "-share"
	capNoRealtime = "-realtime"
)

// importAccounts hands every account of the version before to the sign-in,
// once, on its first start.
func (h handlers) importAccounts(ctx context.Context) ([]signin.Imported, error) {
	all, err := state.GetAllUsers()
	if err != nil {
		return nil, err
	}
	bound, err := h.oldIdentities(ctx)
	if err != nil {
		return nil, err
	}
	codes, err := h.oldRecoveryCodes(ctx)
	if err != nil {
		return nil, err
	}
	profiles, err := h.profiles(ctx)
	if err != nil {
		return nil, err
	}
	owner := ownerOf(all)
	passwords := settings.Config.Auth.Methods.PasswordAuth.Enabled
	o := settings.Config.Auth.Methods.OidcAuth
	out := make([]signin.Imported, 0, len(all))
	for i := range all {
		u := &all[i]
		id := idOf(u)
		in := signin.Imported{
			ID: id, Name: u.Username, Role: roleOf(u), Owner: id == owner,
			Handle: []byte(id), Recovery: codes[id], Seen: unixOrZero(profiles[id].seenAt),
		}
		if u.LoginMethod == users.LoginMethodPassword && passwords {
			in.Password = u.Password
			in.Passkeys = oldPasskeys(u)
		}
		if u.TOTPSecret != "" {
			in.TOTP = oldSecondFactor(u)
		}
		in.Tokens = oldTokens(u)
		for _, b := range bound[id] {
			in.Identities = append(in.Identities, signin.ImportedIdentity{Issuer: b.issuer, Subject: b.subject, Name: u.Username})
		}
		// The version before bound an OIDC account by its name on its first
		// sign-in through the provider of the config.
		if len(bound[id]) == 0 && u.LoginMethod == users.LoginMethodOidc && o.Enabled && !in.Owner {
			in.Identities = append(in.Identities, signin.ImportedIdentity{Issuer: o.IssuerUrl, Name: u.Username, ClaimByName: true})
		}
		out = append(out, in)
	}
	return out, nil
}

// ownerOf is the account that becomes the owner: the admin everyone was
// while the version before was open. A closed server gets no owner, since
// the account of that name may be anyone's by now.
func ownerOf(all []users.User) string {
	if settings.Config.Auth.Methods.OidcAuth.Enabled {
		return ""
	}
	for i := range all {
		u := &all[i]
		if u.LoginMethod == users.LoginMethodPassword && (u.Password != "" || len(u.PasskeyCredentials) > 0) {
			return ""
		}
	}
	u, err := state.ResolveNoAuthUser()
	if err != nil || u.LoginMethod != users.LoginMethodPassword || !u.Permissions.Admin {
		return ""
	}
	return idOf(&u)
}

// oldSecondFactor returns the raw TOTP secret of u. One that does not open
// with the key of the config still asks for a code, so it is imported as
// random bytes that no app knows: recovery codes and the reset get in.
func oldSecondFactor(u *users.User) []byte {
	plain, err := auth.OpenTotpSecret(u.TOTPSecret, u.TOTPNonce)
	var raw []byte
	if err == nil {
		raw, err = base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimRight(plain, "=")))
	}
	if err == nil && len(raw) > 0 {
		return raw
	}
	logger.Warningf("signin: the two-factor secret of %q does not open with auth.totpSecret; a recovery code or a reset gets that account in", u.Username)
	raw = make([]byte, 20)
	_, _ = rand.Read(raw)
	return raw
}

func oldPasskeys(u *users.User) []signin.ImportedPasskey {
	var out []signin.ImportedPasskey
	for _, c := range u.PasskeyCredentials {
		id, err1 := base64.RawURLEncoding.DecodeString(strings.TrimRight(c.ID, "="))
		key, err2 := base64.RawURLEncoding.DecodeString(strings.TrimRight(c.PublicKey, "="))
		if err1 != nil || err2 != nil {
			logger.Warningf("signin: passkey %q of %q does not read and is left out", c.Name, u.Username)
			continue
		}
		aaguid, _ := base64.RawURLEncoding.DecodeString(strings.TrimRight(c.AAGUID, "="))
		rpID := c.Origin
		if rpID == "" {
			if p := publicURL(); p != nil {
				rpID = p.Hostname()
			}
		}
		p := signin.ImportedPasskey{
			Name: c.Name, ID: id, PublicKey: key, SignCount: c.SignCount, AAGUID: aaguid, Transports: c.Transport,
			BackupEligible: c.Flags.BackupEligible, BackupState: c.Flags.BackupState, AttestationType: c.AttestationType,
			RPID: strings.ToLower(rpID), Created: unixOrZero(c.CreatedAt), Used: unixOrZero(c.LastUsedAt),
		}
		out = append(out, p)
	}
	return out
}

// oldTokens are the API tokens of u that still sign in: registered, not
// revoked and not run out.
func oldTokens(u *users.User) []signin.ImportedToken {
	var out []signin.ImportedToken
	for name, t := range u.Tokens {
		owner, isSession, ok := state.HashedTokenOwner(t.Token)
		if !ok || isSession || owner != u.ID || state.IsTokenRevoked(t.Token) {
			continue
		}
		var claims users.AuthToken
		if _, err := auth.ParseToken(t.Token, &claims); err != nil {
			continue
		}
		in := signin.ImportedToken{Name: name, Scopes: tokenScopes(t), Digest: utils.HashSHA256(t.Token)}
		in.Created, in.Expires = claimTimes(claims.RegisteredClaims)
		out = append(out, in)
	}
	return out
}

func claimTimes(c jwt.RegisteredClaims) (created, expires time.Time) {
	if c.IssuedAt != nil {
		created = c.IssuedAt.Time
	}
	if c.ExpiresAt != nil {
		expires = c.ExpiresAt.Time
	}
	if !created.IsZero() && expires.Sub(created) >= neverExpires {
		expires = time.Time{}
	}
	return created, expires
}

// tokenScopes turns what a token of the version before was allowed into
// scopes: read or all, and for a token with caps of its own a scope for
// every right it did not have.
func tokenScopes(t users.AuthToken) []string {
	scopes := []string{scopeAll}
	if t.ReadOnly {
		scopes = []string{scopeRead}
	}
	if users.IsMinimalApiToken(t) {
		return scopes
	}
	caps := users.GlobalPermissionsOnly(t.Permissions)
	for _, c := range []struct {
		has   bool
		scope string
	}{{caps.Admin, capNoAdmin}, {caps.Api, capNoAPI}, {caps.Share, capNoShare}, {caps.Realtime, capNoRealtime}} {
		if !c.has {
			scopes = append(scopes, c.scope)
		}
	}
	return scopes
}

func unixOrZero(s int64) time.Time {
	if s == 0 {
		return time.Time{}
	}
	return time.Unix(s, 0)
}

type oldIdentity struct{ issuer, subject string }

func (h handlers) oldIdentities(ctx context.Context) (map[string][]oldIdentity, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT issuer, subject, user_id FROM oidc_identities`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]oldIdentity{}
	for rows.Next() {
		var b oldIdentity
		var id string
		if err := rows.Scan(&b.issuer, &b.subject, &id); err != nil {
			return nil, err
		}
		out[id] = append(out[id], b)
	}
	return out, rows.Err()
}

func (h handlers) oldRecoveryCodes(ctx context.Context) (map[string][]string, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT user_id, code_hash FROM recovery_codes WHERE used_at = 0 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]string{}
	for rows.Next() {
		var id, hash string
		if err := rows.Scan(&id, &hash); err != nil {
			return nil, err
		}
		out[id] = append(out[id], hash)
	}
	return out, rows.Err()
}

// userByKey finds a user by id or by login name. A login name starts with
// a letter, so the two never mix up.
func userByKey(key string) (users.User, error) {
	if id, err := strconv.ParseUint(key, 10, 64); err == nil {
		return state.GetUserByID(id)
	}
	return state.GetUserByUsername(key)
}
