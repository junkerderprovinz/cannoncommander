package accounts

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
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
// hands out credentials of its own revokes them there. It is meant for init
// functions.
func OnDelete(f func(ctx context.Context, d v1.Deps, userID uint64) error) {
	onDelete = append(onDelete, f)
}

// ended holds what other packages end with the sessions of an account and
// with its API tokens.
var ended struct {
	sessions, tokens []func(ctx context.Context, db *sql.DB, userID uint64) error
}

// OnSessionsEnd adds f to what ends when every session of an account ends,
// as after a new password, for a credential that came in through a session.
// It is meant for init functions.
func OnSessionsEnd(f func(ctx context.Context, db *sql.DB, userID uint64) error) {
	ended.sessions = append(ended.sessions, f)
}

// OnTokensEnd adds f to what ends with the API tokens of an account, for a
// credential that lasts like one. It is meant for init functions.
func OnTokensEnd(f func(ctx context.Context, db *sql.DB, userID uint64) error) {
	ended.tokens = append(ended.tokens, f)
}

// endSessions logs u out everywhere but the session keep, along with what
// came in through a session.
func endSessions(ctx context.Context, db *sql.DB, u *users.User, keep string) (int, error) {
	n, err := login.EndAll(u.ID, keep)
	if err != nil {
		return n, err
	}
	for _, f := range ended.sessions {
		if err := f(ctx, db, u.ID); err != nil {
			return n, err
		}
	}
	return n, nil
}

// deleteUser removes an account with everything that hangs on it: shares,
// jobs, undo steps, rules, settings and sessions. Files stay where they are,
// and its trash stays for admins until the clean up deletes it.
func (h handlers) deleteUser(w http.ResponseWriter, r *http.Request) error {
	u, err := target(r)
	if err != nil {
		return err
	}
	actor := v1.UserFrom(r)
	if u.ID == actor.ID {
		return v1.Coded(http.StatusForbidden, "deleteSelf", "you cannot delete your own account")
	}
	// What hangs on the login name or grants access goes first, so a new
	// account with the same name can never inherit it.
	if err := shares.RemoveUser(r.Context(), h.deps, u.ID); err != nil {
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
	if err := fileops.For(h.deps).ForgetOwner(r.Context(), u.Username); err != nil {
		return err
	}
	if err := h.svc.rules.dropWho(r.Context(), u.Username); err != nil {
		return err
	}
	for _, f := range onDelete {
		if err := f(r.Context(), h.deps, u.ID); err != nil {
			return err
		}
	}
	if err := state.DeleteUser(u.ID); err != nil {
		return err
	}
	// The rest is keyed by an id that is never handed out again, so a
	// failure here only leaves rows nobody can reach.
	if err := forgetRows(r.Context(), h, u.ID); err != nil {
		logger.Errorf("clean up after user %s: %v", u.Username, err)
	}
	activity.RecordUserMutation(r, &activity.Actor{User: actor}, activitydb.EventUserDelete, &u, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// moveName carries what hangs on a login name over to the new one. It tries
// everything even when one step fails, as the account is renamed already.
func (h handlers) moveName(ctx context.Context, from, to string) error {
	var jobErr error
	if m := jobs.For(h.deps); m != nil {
		jobErr = m.Rename(from, to)
	}
	return errors.Join(h.svc.rules.renameWho(ctx, from, to), jobErr, undo.For(h.deps).Rename(from, to), trash.For(h.deps).Rename(from, to),
		fileops.For(h.deps).RenameOwner(ctx, from, to))
}

func forgetRows(ctx context.Context, h handlers, userID uint64) error {
	if err := prefs.Forget(ctx, h.db, userID); err != nil {
		return err
	}
	id := strconv.FormatUint(userID, 10)
	for _, t := range ownTables {
		if _, err := h.db.ExecContext(ctx, `DELETE FROM `+t+` WHERE user_id = ?`, id); err != nil {
			return err
		}
	}
	return nil
}
