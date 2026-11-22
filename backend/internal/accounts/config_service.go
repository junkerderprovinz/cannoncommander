package accounts

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// configServiceID is the sign-in service of the OIDC section in config.yaml.
const configServiceID = "config"

// The provider knows the paths the version before used, so the service
// keeps them.
const (
	oidcStartPath    = "/api/auth/oidc/login"
	oidcCallbackPath = "/api/auth/oidc/callback"
)

// configService is the OIDC of config.yaml as a sign-in service, or nil
// when it is off. What the sign-in cannot honour stops the start with the
// line to change, since running on would lock everyone out.
func configService() (*signin.ServiceSpec, error) {
	o := settings.Config.Auth.Methods.OidcAuth
	if !o.Enabled {
		return nil, nil
	}
	if o.DisableVerifyTLS {
		return nil, errors.New("auth.methods.oidc.disableVerifyTLS cannot be used: put the certificate of the provider's authority in a file and name it in auth.methods.oidc.caFile")
	}
	callback := settings.Config.Http.BaseURL + strings.TrimPrefix(oidcCallbackPath, "/")
	if u := publicURL(); u == nil || u.Scheme != "https" {
		return nil, fmt.Errorf("OIDC needs http.externalUrl, the https address people open CannonCommander at, and the provider must send them back to <that address>%s", callback)
	}
	spec := &signin.ServiceSpec{
		ID: configServiceID, Provider: o.Provider, Name: o.Name,
		Issuer: o.IssuerUrl, ClientID: o.ClientID, Secret: o.ClientSecret,
		Scopes: o.Scopes, NameClaim: o.UserIdentifier, GroupsClaim: o.GroupsClaim,
		StartPath: oidcStartPath, CallbackPath: oidcCallbackPath,
	}
	if o.CaFile != "" {
		pem, err := os.ReadFile(o.CaFile)
		if err != nil {
			return nil, fmt.Errorf("auth.methods.oidc.caFile: %w", err)
		}
		spec.CA = string(pem)
	}
	return spec, nil
}

var errOutsideUserGroups = errors.New("not in any of auth.methods.oidc.userGroups")

// admit applies userGroups and adminGroup of config.yaml to the config
// service, on every sign-in and renewal. Services added in the app give
// newcomers the role chosen in their form.
func admit(_ context.Context, in signin.Arrival) (string, error) {
	if in.Service != configServiceID {
		return "", nil
	}
	o := settings.Config.Auth.Methods.OidcAuth
	if len(o.UserGroups) > 0 && !slices.ContainsFunc(in.Groups, func(g string) bool { return slices.Contains(o.UserGroups, g) }) {
		return "", errOutsideUserGroups
	}
	inAdmins := o.AdminGroup != "" && slices.Contains(in.Groups, o.AdminGroup)
	switch {
	case in.Account == "" && inAdmins:
		return roleAdmin, nil
	case in.Account == "":
		return roleUser, nil
	case o.AdminGroup == "" || in.Groups == nil:
		return "", nil
	case inAdmins:
		return roleAdmin, nil
	case in.Role == roleAdmin:
		return roleUser, nil
	}
	return "", nil
}

// checkNames stops the start when two accounts have names that differ only
// in case, which the sign-in cannot tell apart.
func checkNames() error {
	all, err := state.GetAllUsers()
	if err != nil {
		return err
	}
	seen := map[string]users.User{}
	for _, u := range all {
		key := strings.ToLower(strings.TrimSpace(u.Username))
		if other, ok := seen[key]; ok {
			return fmt.Errorf("the accounts %q (id %d) and %q (id %d) differ only in case; start once with %s=<id>:<new name> to rename one", other.Username, other.ID, u.Username, u.ID, renameEnv)
		}
		seen[key] = u
	}
	return nil
}

const renameEnv = "CC_SIGNIN_RENAME"

// renameOnce renames the account CC_SIGNIN_RENAME names before the import,
// with its rules and groups, so a clash of names can be solved without the
// older version.
func renameOnce(ctx context.Context, h handlers) error {
	ask := os.Getenv(renameEnv)
	if ask == "" {
		return nil
	}
	id, name, ok := strings.Cut(ask, ":")
	u, err := state.GetUserByUsername(name)
	if ok && err == nil && fmt.Sprint(u.ID) == id {
		return nil
	}
	target, err := userByKey(id)
	if !ok || err != nil || strings.TrimSpace(name) == "" {
		return fmt.Errorf("%s=%q names no account as <id>:<new name>", renameEnv, ask)
	}
	old := target.Username
	target.Username = strings.TrimSpace(name)
	if err := state.UpdateUser(&target, "", "username"); err != nil {
		return fmt.Errorf("%s: %w", renameEnv, err)
	}
	if err := h.moveName(ctx, old, target.Username); err != nil {
		return err
	}
	logger.Warningf("%s: renamed %q to %q", renameEnv, old, target.Username)
	return nil
}

// warnAboutConfig names what config.yaml asks for that the sign-in does
// differently.
func warnAboutConfig(ctx context.Context, h handlers) {
	m := settings.Config.Auth.Methods
	if m.PasswordAuth.EnforcedOtp {
		logger.Warning("auth.methods.password.enforcedOtp has no effect any more; each account turns on two-factor for itself")
	}
	if !m.PasswordAuth.Enabled {
		all, _ := state.GetAllUsers()
		var names []string
		for _, u := range all {
			if u.LoginMethod == users.LoginMethodPassword && (u.Password != "" || len(u.PasskeyCredentials) > 0) {
				names = append(names, u.Username)
			}
		}
		if len(names) > 0 {
			logger.Warningf("auth.methods.password.enabled is false, so these accounts cannot sign in with their password or passkeys: %s", strings.Join(names, ", "))
		}
	}
	if settings.ConfigTriggersAdminPasswordReset() {
		logger.Warning("auth.methods.password.adminPassword only sets the first password; a forgotten one is reset through the log")
	}
	if !m.OidcAuth.Enabled || h.db == nil {
		return
	}
	rows, err := h.db.QueryContext(ctx, `SELECT DISTINCT issuer FROM oidc_identities WHERE issuer <> ?`, m.OidcAuth.IssuerUrl)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var issuer string
		if rows.Scan(&issuer) == nil {
			logger.Warningf("accounts bound to the OIDC issuer %q do not sign in through %q and are not moved over by name", issuer, m.OidcAuth.IssuerUrl)
		}
	}
}
