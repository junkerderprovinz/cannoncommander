package accounts

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/access"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	cerrors "github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// The hooks run while the sign-in holds its lock. None of them calls the
// sign-in, and no code here holds the lock of users, groups or rules while
// it calls the sign-in, so the two never wait for each other.

// newUser is what the Users tile picks for an account it creates, which
// the sign-in hands back to changing in the request's context.
type newUser struct {
	scopes []users.BackendScope
}

type newUserKey struct{}

// changing writes the users row of an account before the sign-in writes
// the account, so every table keyed by its id or name points at it.
func (h handlers) changing(ctx context.Context, c signin.Change) error {
	id, err := strconv.ParseUint(c.Account, 10, 64)
	if err != nil {
		return err
	}
	switch c.Kind {
	case signin.ChangeCreate:
		return h.created(ctx, id, c)
	case signin.ChangeRename:
		u, err := state.GetUserByID(id)
		if err != nil {
			return err
		}
		old := u.Username
		u.Username = c.Name
		if err := state.UpdateUser(&u, "", "username"); err != nil {
			return err
		}
		return h.moveName(ctx, old, c.Name)
	case signin.ChangeRole:
		u, err := state.GetUserByID(id)
		if err != nil {
			return err
		}
		u.Permissions.Admin, u.Permissions.Share = c.Role == roleAdmin, mayShareAs(c.Role)
		u.Permissions = users.MarkPermissionsConfigured(u.Permissions)
		u.BackendScopes = h.withRole(u.BackendScopes, c.Role)
		return state.UpdateUser(&u, "", "permissions", "backendScopes")
	}
	return nil
}

// created writes the row of a new account. An account from the service in
// config.yaml is an OIDC account bound to its identity, as the version
// before made them. Every other one gets a password nobody knows, so that
// version neither counts it as open nor hands it to an OIDC sign-in of the
// same name.
func (h handlers) created(ctx context.Context, id uint64, c signin.Change) error {
	// The sign-in holds no account of this name, so a row with it is one an
	// earlier create left when the sign-in could not write its account.
	if old, err := state.GetUserByUsername(c.Name); err == nil {
		if err := state.DeleteUser(old.ID); err != nil {
			return err
		}
		if err := forgetRows(ctx, h, old.ID); err != nil {
			return err
		}
	}
	perms := users.MarkPermissionsConfigured(users.Permissions{Admin: c.Role == roleAdmin, Share: mayShareAs(c.Role)})
	u := users.User{ID: id, FrontendUser: users.FrontendUser{Username: c.Name, LoginMethod: users.LoginMethodPassword, Permissions: perms}}
	fromConfig := c.Via == signin.ViaService && c.Service == configServiceID
	password := ""
	if fromConfig {
		u.LoginMethod = users.LoginMethodOidc
	} else {
		password = unusablePassword()
	}
	plan, _ := ctx.Value(newUserKey{}).(*newUser)
	if err := state.CreateUser(&u, password); err != nil {
		return err
	}
	// Creating adds the default sources. The scopes the tile picked are the
	// ones that count, and otherwise the defaults get the role's rights.
	if plan != nil {
		u.BackendScopes = plan.scopes
	} else {
		u.BackendScopes = h.withRole(u.BackendScopes, c.Role)
	}
	u.Permissions = perms
	if err := state.UpdateUser(&u, "", "backendScopes", "permissions"); err != nil {
		return errors.Join(err, state.DeleteUser(id))
	}
	if !fromConfig {
		return nil
	}
	return state.InLegacyTx(func(tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT OR REPLACE INTO oidc_identities (issuer, subject, user_id) VALUES (?, ?, ?)`, c.Issuer, c.Subject, c.Account)
		return err
	})
}

// signedIn brings the groups a sign-in service reported into CC's groups.
func (h handlers) signedIn(ctx context.Context, v signin.Visitor, groups []string) {
	if v.Via != signin.ViaService || groups == nil {
		return
	}
	if err := h.syncGroups(ctx, v.Name, groups); err != nil {
		logger.Errorf("groups of %s from the sign-in service: %v", v.Name, err)
	}
}

// syncGroups makes name a member of exactly groups among the memberships a
// sign-in service gave. One an admin gave stays as it is.
func (h handlers) syncGroups(ctx context.Context, name string, groups []string) error {
	rows, err := h.db.QueryContext(ctx, `SELECT group_name FROM synced_members WHERE username = ?`, name)
	if err != nil {
		return err
	}
	var synced []string
	for rows.Next() {
		var g string
		if err := rows.Scan(&g); err != nil {
			rows.Close()
			return err
		}
		synced = append(synced, g)
	}
	rows.Close()
	want := slices.DeleteFunc(slices.Clone(groups), func(g string) bool { return !access.IsUsableSyncedGroupName(g) })
	for _, g := range synced {
		if slices.Contains(want, g) {
			continue
		}
		if err := state.RemoveUserFromGroup(g, name); err != nil {
			return err
		}
		if _, err := h.db.ExecContext(ctx, `DELETE FROM synced_members WHERE group_name = ? AND username = ?`, g, name); err != nil {
			return err
		}
	}
	have := state.GetUserGroups(name)
	for _, g := range want {
		if slices.Contains(have, g) {
			continue
		}
		if err := state.AddUserToGroup(g, name); err != nil {
			return err
		}
		if _, err := h.db.ExecContext(ctx, `INSERT OR IGNORE INTO synced_members (group_name, username) VALUES (?, ?)`, g, name); err != nil {
			return err
		}
	}
	return nil
}

// ended ends what CC keeps for itself when the sign-in ended ways into
// account. It runs again until it returns nil, so every step takes what is
// gone already as done.
func (h handlers) ended(ctx context.Context, account string, what signin.Ending) error {
	id, err := strconv.ParseUint(account, 10, 64)
	if err != nil {
		return nil
	}
	basicSeen.forget(account)
	switch what {
	case signin.EndRemoved:
		return h.removed(ctx, id)
	case signin.EndBlocked:
		// The sign-in drops the old fields of an imported account only on
		// its first ending, and accounts it made have none to drop.
		if err := shutOut(ctx, account, everyOldWay); err != nil {
			return err
		}
	}
	switch what {
	case signin.EndFirstPassword, signin.EndPassword, signin.EndReset:
		if err := mirrorPassword(ctx, account); err != nil {
			return err
		}
		if err := passwordChanged(ctx, h.db, account); err != nil {
			return err
		}
		if u, err := state.GetUserByID(id); err == nil && u.Username == settings.PasswordAdminUsername() {
			state.ForgetFirstAdminPassword()
		}
	}
	// A client that came in through OAuth came through a session, so it
	// goes with every ending. A key lasts like an API token and goes when
	// they do, and with whatever was set up while the server was open.
	for _, f := range ended.sessions {
		if err := f(ctx, h.db, id); err != nil {
			return err
		}
	}
	switch what {
	case signin.EndPassword, signin.EndEverywhere, signin.EndTwoFactor:
		return nil
	}
	for _, f := range ended.tokens {
		if err := f(ctx, h.db, id); err != nil {
			return err
		}
	}
	return nil
}

// removed takes everything that hangs on an account away, the parts keyed
// by its name first, so a new account of that name inherits nothing. The
// sign-in keeps the name taken until this returns nil.
func (h handlers) removed(ctx context.Context, id uint64) error {
	u, err := state.GetUserByID(id)
	if errors.Is(err, cerrors.ErrNotExist) {
		return forgetRows(ctx, h, id)
	}
	if err != nil {
		return err
	}
	// A hook that keeps failing must not leave a row the version before
	// still lets in.
	if err := shutOut(ctx, strconv.FormatUint(id, 10), everyOldWay); err != nil {
		return err
	}
	if err := h.forget(ctx, &u); err != nil {
		return err
	}
	if err := state.DeleteUser(id); err != nil {
		return err
	}
	return forgetRows(ctx, h, id)
}

// logged keeps the sign-ins and what they changed in the activity log.
func logged(_ context.Context, e signin.Entry) {
	events := map[string]activitydb.EventType{
		"ok": activitydb.EventLogin, "newAccount": activitydb.EventSignup,
		"passkeyAdded": activitydb.EventPasskeyRegister, "tokenMade": activitydb.EventTokenCreate,
	}
	event, ok := events[e.Result]
	id, err := strconv.ParseUint(e.Account, 10, 64)
	if !ok || err != nil {
		return
	}
	at := e.At
	if at.IsZero() {
		at = time.Now()
	}
	activity.Record(activitydb.Entry{
		UserID: id, EventType: event, IPAddress: e.From, CreatedAt: at.Unix(),
		Details: activitydb.Details{LoginMethod: string(e.Via)},
	})
}
