package accounts

import (
	"context"
	"database/sql"
	"errors"
	"strconv"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// ownTables are the tables of this package with rows per user.
var ownTables = []string{"sessions", "account_profiles", "recovery_codes", "token_use", "oidc_identities"}

// onDelete holds what other packages drop when an account goes.
var onDelete []func(ctx context.Context, d v1.Deps, userID uint64) error

// OnDelete adds f to what runs before an account is deleted. A package that
// hands out credentials of its own revokes them there. It runs again when
// it fails, so f takes what is gone already as done. It is meant for init
// functions.
func OnDelete(f func(ctx context.Context, d v1.Deps, userID uint64) error) {
	onDelete = append(onDelete, f)
}

// ended holds what other packages end with the sessions of an account and
// with its API tokens.
var ended struct {
	sessions, tokens []func(ctx context.Context, db *sql.DB, userID uint64) error
}

// OnSessionsEnd adds f to what ends with the sessions of an account, for a
// credential that came in through a session. It is meant for init
// functions.
func OnSessionsEnd(f func(ctx context.Context, db *sql.DB, userID uint64) error) {
	ended.sessions = append(ended.sessions, f)
}

// OnTokensEnd adds f to what ends with the API tokens of an account, for a
// credential that lasts like one. It is meant for init functions.
func OnTokensEnd(f func(ctx context.Context, db *sql.DB, userID uint64) error) {
	ended.tokens = append(ended.tokens, f)
}

// forget drops what hangs on the login name of u or grants access: shares,
// jobs, undo steps, the trash it owns, rules and keys. Files stay where they
// are, and the trash stays for admins until the clean up deletes it.
func (h handlers) forget(ctx context.Context, u *users.User) error {
	if err := shares.RemoveUser(ctx, h.deps, u.ID); err != nil {
		return err
	}
	if m := jobs.For(h.deps); m != nil {
		m.Forget(u.Username)
	}
	if err := undo.For(h.deps).Forget(u.Username); err != nil {
		return err
	}
	if err := trash.For(h.deps).Disown(u.Username); err != nil {
		return err
	}
	if err := fileops.For(h.deps).ForgetOwner(ctx, u.Username); err != nil {
		return err
	}
	if err := h.svc.rules.dropWho(ctx, u.Username); err != nil {
		return err
	}
	for _, f := range onDelete {
		if err := f(ctx, h.deps, u.ID); err != nil {
			return err
		}
	}
	_, err := h.db.ExecContext(ctx, `DELETE FROM synced_members WHERE username = ?`, u.Username)
	return err
}

// moveName carries what hangs on a login name over to the new one. It tries
// everything even when one step fails, as the account is renamed already.
func (h handlers) moveName(ctx context.Context, from, to string) error {
	var jobErr error
	if m := jobs.For(h.deps); m != nil {
		jobErr = m.Rename(from, to)
	}
	_, syncErr := h.db.ExecContext(ctx, `UPDATE synced_members SET username = ? WHERE username = ?`, to, from)
	return errors.Join(h.svc.rules.renameWho(ctx, from, to), jobErr, undo.For(h.deps).Rename(from, to), trash.For(h.deps).Rename(from, to),
		fileops.For(h.deps).RenameOwner(ctx, from, to), syncErr)
}

// forgetRows drops the rows keyed by an id that is never handed out again.
func forgetRows(ctx context.Context, h handlers, userID uint64) error {
	if err := prefs.Forget(ctx, h.db, userID); err != nil {
		return err
	}
	id := strconv.FormatUint(userID, 10)
	return state.InLegacyTx(func(tx *sql.Tx) error {
		for _, t := range ownTables {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+t+` WHERE user_id = ?`, id); err != nil {
				return err
			}
		}
		return nil
	})
}
