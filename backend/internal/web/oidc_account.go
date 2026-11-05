package web

import (
	"context"
	"database/sql"
	libError "errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/files"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// oidcIdentity is the person an identity provider vouches for. Issuer and
// subject stay the same for that person, the name can change or pass to
// someone else.
type oidcIdentity struct {
	issuer, subject, name string
	// groups is nil when the provider sent none.
	groups []string
}

var errOidcNameTaken = libError.New("the account with this name belongs to another person at the identity provider")

// oidcAccount returns the account of id: the one bound to it, or for a first
// sign in the account of that name, which is bound from then on. A name
// nobody has gets a new account.
func oidcAccount(ctx context.Context, db *sql.DB, id oidcIdentity, admin bool) (users.User, error) {
	userID, bound, err := accounts.OIDCOwner(ctx, db, id.issuer, id.subject)
	if err != nil {
		return users.User{}, err
	}
	if bound {
		u, err := state.GetUserByID(userID)
		if err == nil {
			return u, checkOidcMethod(&u)
		}
		if !libError.Is(err, errors.ErrNotExist) {
			return users.User{}, err
		}
	}

	u, err := state.GetUserByUsername(id.name)
	switch {
	case libError.Is(err, errors.ErrNotExist):
		if u, err = createOidcUser(id.name, admin); err != nil {
			return users.User{}, err
		}
	case err != nil:
		return users.User{}, err
	default:
		if err := checkOidcMethod(&u); err != nil {
			return users.User{}, err
		}
		// An account bound under an earlier issuer moves to this one by name,
		// as it did when the bindings came in.
		issuer, err := accounts.OIDCIssuerOf(ctx, db, u.ID)
		if err != nil {
			return users.User{}, err
		}
		if issuer == id.issuer {
			return users.User{}, errOidcNameTaken
		}
	}
	return u, accounts.BindOIDC(ctx, db, u.ID, id.issuer, id.subject)
}

// checkOidcMethod refuses accounts that sign in elsewhere. A name at the
// identity provider can match a local account it has nothing to do with.
func checkOidcMethod(u *users.User) error {
	if u.LoginMethod != users.LoginMethodOidc {
		logger.Debugf("login rejected: OIDC sign in for %q, whose account signs in through %q", u.Username, u.LoginMethod)
		return errors.ErrWrongLoginMethod
	}
	return nil
}

func createOidcUser(name string, admin bool) (users.User, error) {
	u := users.User{FrontendUser: users.FrontendUser{LoginMethod: users.LoginMethodOidc, Username: name}}
	u.Permissions.Admin = admin
	if err := state.CreateUser(&u, ""); err != nil {
		return users.User{}, err
	}
	if err := files.MakeUserDirs(&u, true); err != nil {
		logger.Error(err.Error())
	}
	return state.GetUserByUsername(name)
}

// syncOidcRights applies what the provider says about u. A nil admin leaves
// the admin right as it is, nil groups leave the memberships.
func syncOidcRights(u *users.User, admin *bool, groups []string) error {
	if admin != nil && u.Permissions.Admin != *admin {
		u.Permissions.Admin = *admin
		if err := state.UpdateUser(u, "", "permissions"); err != nil {
			return err
		}
	}
	if groups != nil {
		if err := state.SyncUserGroups(u.Username, groups); err != nil {
			return fmt.Errorf("sync the groups of %s: %w", u.Username, err)
		}
	}
	return nil
}

// endOidcAccess ends the sessions and API tokens of the account bound to id,
// whom the provider vouches for but who may not sign in here any more.
// Nothing tells the app when the provider removes someone, so this happens
// at their next sign in.
func endOidcAccess(r *http.Request, db *sql.DB, id oidcIdentity) {
	userID, bound, err := accounts.OIDCOwner(r.Context(), db, id.issuer, id.subject)
	if err != nil || !bound {
		return
	}
	u, err := state.GetUserByID(userID)
	if err != nil {
		return
	}
	if err := accounts.EndAccess(r, db, &u); err != nil {
		logger.Errorf("end the access of %s: %v", u.Username, err)
		return
	}
	logger.Warningf("ended the sessions and API tokens of %s, the identity provider no longer lets them in", u.Username)
}

func inAnyGroup(groups, allowed []string) bool {
	return slices.ContainsFunc(allowed, func(g string) bool { return slices.Contains(groups, g) })
}
