package accounts

import (
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// The rights a token can have. It never gets more than its owner.
const (
	rightsRead      = "r"
	rightsReadWrite = "rw"
)

// forever is how long a token lives that should never run out. The login
// system needs an expiry on every token.
const forever = 100 * 365 * 24 * time.Hour

// Token is an API token as the list shows it, without its secret.
type Token struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Rights  string     `json:"rights"`
	Created time.Time  `json:"created"`
	Expires *time.Time `json:"expires"`
	Last    *time.Time `json:"last"`
}

// NewToken is a token right after it was made. The secret is shown this
// once and never again.
type NewToken struct {
	Token
	Secret string `json:"secret"`
}

// tokenID names a token in URLs. It comes from the secret, so it stays put
// while names may hold any character.
func tokenID(t users.AuthToken) string { return utils.HashSHA256(t.Token)[:16] }

func rightsOf(t users.AuthToken) string {
	if t.ReadOnly {
		return rightsRead
	}
	return rightsReadWrite
}

func tokenTimes(t users.AuthToken) (created time.Time, expires *time.Time) {
	created = time.Unix(t.IssuedAt, 0).UTC()
	if t.RegisteredClaims.IssuedAt != nil {
		created = t.RegisteredClaims.IssuedAt.UTC()
	}
	end := time.Unix(t.ExpiresAt, 0).UTC()
	if t.RegisteredClaims.ExpiresAt != nil {
		end = t.RegisteredClaims.ExpiresAt.UTC()
	}
	if end.Sub(created) < forever/2 {
		expires = &end
	}
	return created, expires
}

func (h handlers) tokenUse(r *http.Request, u *users.User) (map[string]int64, error) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT token_hash, seen_at FROM token_use WHERE user_id = ?`, idOf(u))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var hash string
		var at int64
		if err := rows.Scan(&hash, &at); err != nil {
			return nil, err
		}
		out[hash] = at
	}
	return out, rows.Err()
}

func (h handlers) listTokens(w http.ResponseWriter, r *http.Request) error {
	u, err := current(r)
	if err != nil {
		return err
	}
	return h.writeTokens(w, r, &u)
}

func (h handlers) writeTokens(w http.ResponseWriter, r *http.Request, u *users.User) error {
	used, err := h.tokenUse(r, u)
	if err != nil {
		return err
	}
	items := []Token{}
	users.EachNamedToken(u.Tokens, func(name string, t users.AuthToken) {
		created, expires := tokenTimes(t)
		items = append(items, Token{ID: tokenID(t), Name: name, Rights: rightsOf(t), Created: created, Expires: expires, Last: unixTime(used[utils.HashSHA256(t.Token)])})
	})
	slices.SortFunc(items, func(a, b Token) int { return strings.Compare(a.Name, b.Name) })
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Token{"items": items})
}

// tokenByID finds the API token of u that the list shows with this id.
func tokenByID(u *users.User, id string) (users.AuthToken, error) {
	var tok users.AuthToken
	users.EachNamedToken(u.Tokens, func(_ string, t users.AuthToken) {
		if tokenID(t) == id {
			tok = t
		}
	})
	if tok.Name == "" {
		return tok, v1.Coded(http.StatusNotFound, "noToken", "there is no such token")
	}
	return tok, nil
}

type tokenRequest struct {
	Name   string `json:"name"`
	Rights string `json:"rights"`
	// Days is how long the token lives, null or 0 for always.
	Days     *int   `json:"days"`
	Password string `json:"password"`
}

func (h handlers) createToken(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	var in tokenRequest
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(in.Name)
	switch {
	case name == "":
		return v1.Coded(http.StatusBadRequest, "tokenNameMissing", "give the token a name")
	// Session tokens carry the WEB_TOKEN names, and a name with two dots could
	// pass for a token itself among the keys of the token map.
	case len(name) > 64 || strings.HasPrefix(name, "WEB_TOKEN") || strings.Count(name, ".") == 2:
		return v1.Coded(http.StatusBadRequest, "tokenNameReserved", "pick another name for the token")
	case in.Rights != rightsRead && in.Rights != rightsReadWrite:
		return v1.Errorf(http.StatusBadRequest, "the rights have to be r or rw")
	case in.Days != nil && (*in.Days < 0 || *in.Days > 3650):
		return v1.Errorf(http.StatusBadRequest, "a token lives between 1 and 3650 days, or 0 for always")
	}
	u, err := current(r)
	if err != nil {
		return err
	}
	if err := h.confirmOwner(r, &u, in.Password); err != nil {
		return err
	}
	if _, taken := u.Tokens[name]; taken {
		return v1.Coded(http.StatusConflict, "tokenExists", "there is a token with this name already")
	}
	life := forever
	if in.Days != nil && *in.Days > 0 {
		life = time.Duration(*in.Days) * 24 * time.Hour
	}
	raw, tok, err := auth.MakeSignedTokenAPI(&u, name, life, u.Permissions, true)
	if err != nil {
		return err
	}
	tok.Name, tok.Token, tok.ReadOnly = name, raw, in.Rights == rightsRead
	tok.IssuedAt, tok.ExpiresAt = tok.RegisteredClaims.IssuedAt.Unix(), tok.RegisteredClaims.ExpiresAt.Unix()
	if err := state.AddUserToken(u.Username, tok); err != nil {
		return err
	}
	if err := state.AddApiToken(raw, u.ID); err != nil {
		if rmErr := state.DeleteUserToken(u.Username, name); rmErr != nil {
			logger.Errorf("roll back token %s: %v", name, rmErr)
		}
		return err
	}
	activity.RecordTokenMutation(r, &activity.Actor{User: &u}, activitydb.EventTokenCreate, name)
	created, expires := tokenTimes(tok)
	return v1.WriteJSON(w, r, http.StatusCreated, NewToken{
		Token:  Token{ID: tokenID(tok), Name: name, Rights: in.Rights, Created: created, Expires: expires},
		Secret: raw,
	})
}

func (h handlers) deleteToken(w http.ResponseWriter, r *http.Request) error {
	if err := needSession(r); err != nil {
		return err
	}
	u, err := current(r)
	if err != nil {
		return err
	}
	tok, err := tokenByID(&u, r.PathValue("id"))
	if err != nil {
		return err
	}
	if err := h.revokeToken(r, &u, tok); err != nil {
		return err
	}
	activity.RecordTokenMutation(r, &activity.Actor{User: &u}, activitydb.EventTokenDelete, tok.Name)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// revokeTokens ends every API token of u.
func (h handlers) revokeTokens(r *http.Request, u *users.User) error {
	var all []users.AuthToken
	users.EachNamedToken(u.Tokens, func(_ string, t users.AuthToken) { all = append(all, t) })
	for _, t := range all {
		if err := h.revokeToken(r, u, t); err != nil {
			return err
		}
	}
	for _, f := range ended.tokens {
		if err := f(r.Context(), h.db, u.ID); err != nil {
			return err
		}
	}
	return nil
}

// revokeToken ends a token at once and forgets it.
func (h handlers) revokeToken(r *http.Request, u *users.User, tok users.AuthToken) error {
	if err := state.DeleteUserToken(u.Username, tok.Name); err != nil {
		return err
	}
	if err := state.RevokeToken(tok.Token); err != nil {
		return err
	}
	if err := state.RemoveApiToken(tok.Token); err != nil {
		return err
	}
	_, err := h.db.ExecContext(r.Context(), `DELETE FROM token_use WHERE token_hash = ?`, utils.HashSHA256(tok.Token))
	return err
}
