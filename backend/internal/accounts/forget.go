package accounts

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// ownTables are the tables of this package with rows per user.
var ownTables = []string{"sessions", "account_profiles", "recovery_codes", "token_use"}

// deleteUser removes an account with everything that hangs on it: shares,
// jobs, rules, settings and sessions. Files stay where they are.
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
	// Links made before the app had its own still live in the login system.
	old, err := state.GetSharesByUserID(u.ID)
	if err != nil {
		return err
	}
	for _, s := range old {
		if err := state.DeleteShare(s.Hash); err != nil {
			return err
		}
	}
	if m := jobs.For(h.deps); m != nil {
		m.Forget(u.Username)
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
