package accounts

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/migrate"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func init() {
	// Accounts that signed in through OIDC before this table existed have no
	// row. Their next sign in binds them, found by name as before.
	migrate.Register(migrate.Main, migrate.SQL("accounts/008_oidc_identities",
		`CREATE TABLE oidc_identities (
			issuer TEXT NOT NULL,
			subject TEXT NOT NULL,
			user_id TEXT NOT NULL UNIQUE,
			PRIMARY KEY (issuer, subject)
		)`,
	))
}

// OIDCOwner returns the account bound to the identity the provider issuer
// knows as subject. ok is false when no account is.
func OIDCOwner(ctx context.Context, db *sql.DB, issuer, subject string) (userID uint64, ok bool, err error) {
	var id string
	err = db.QueryRowContext(ctx, `SELECT user_id FROM oidc_identities WHERE issuer = ? AND subject = ?`, issuer, subject).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	userID, err = strconv.ParseUint(id, 10, 64)
	return userID, err == nil, err
}

// OIDCIssuerOf returns the provider the account is bound to, or "" for an
// account that is not bound.
func OIDCIssuerOf(ctx context.Context, db *sql.DB, userID uint64) (string, error) {
	var issuer string
	err := db.QueryRowContext(ctx, `SELECT issuer FROM oidc_identities WHERE user_id = ?`, strconv.FormatUint(userID, 10)).Scan(&issuer)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return issuer, err
}

// BindOIDC binds the account to the identity, in place of any binding either
// of them had.
func BindOIDC(ctx context.Context, db *sql.DB, userID uint64, issuer, subject string) error {
	_, err := db.ExecContext(ctx, `INSERT OR REPLACE INTO oidc_identities (issuer, subject, user_id) VALUES (?, ?, ?)`,
		issuer, subject, strconv.FormatUint(userID, 10))
	return err
}

// EndAccess ends every session and API token of u, for an account whose
// identity the provider no longer lets in.
func EndAccess(r *http.Request, db *sql.DB, u *users.User) error {
	if _, err := login.EndAll(u.ID, ""); err != nil {
		return err
	}
	return handlers{db: db}.revokeTokens(r, u)
}
