package accounts

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
)

// ownTables are the tables of this package with rows per user.
var ownTables = []string{"sessions", "account_profiles", "recovery_codes", "token_use"}

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
		return v1.Errorf(http.StatusForbidden, "you cannot delete your own account")
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
