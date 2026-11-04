package accounts

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// The roles of the app. Admins may do everything, users work with the files
// in their scope, and read only users look and download.
const (
	roleAdmin    = "admin"
	roleUser     = "user"
	roleReadOnly = "readonly"
)

// minPassword is the shortest password the app takes. The server setting
// can ask for more.
const minPassword = 12

// Account is a user as the app shows it. Name is the login name, Full the
// name to show, and Scope the folder the user sees.
type Account struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Full        string     `json:"full"`
	Role        string     `json:"role"`
	Scope       string     `json:"scope"`
	Groups      []string   `json:"groups"`
	LoginMethod string     `json:"loginMethod"`
	Twofa       bool       `json:"twofa"`
	Passkeys    int        `json:"passkeys"`
	Last        *time.Time `json:"last"`
	PwChanged   *time.Time `json:"pwChanged"`
}

type handlers struct {
	db   *sql.DB
	fs   *ccfs.Resolver
	deps v1.Deps
	svc  *service
}

func idOf(u *users.User) string { return strconv.FormatUint(u.ID, 10) }

// current loads the logged in user fresh from the store. The user on the
// request may carry the narrowed rights of an API token, which must never
// be written back.
func current(r *http.Request) (users.User, error) {
	return state.GetUserByID(v1.UserFrom(r).ID)
}

func roleOf(u *users.User) string {
	if u.Permissions.Admin {
		return roleAdmin
	}
	for _, s := range u.BackendScopes {
		p, _ := u.FilePermsForSourcePath(s.Path)
		if p.Create || p.Modify || p.Delete {
			return roleUser
		}
	}
	return roleReadOnly
}

func validRole(role string) bool {
	return role == roleAdmin || role == roleUser || role == roleReadOnly
}

func rolePerms(role string) users.SourceFilePermissions {
	p := users.SourceFilePermissions{View: true, Download: true}
	if role != roleReadOnly {
		p.Create, p.Modify, p.Delete = true, true, true
	}
	return users.MarkSourceFilePermissionsConfigured(p)
}

// scopePaths turns the scopes of u into absolute paths.
func (h handlers) scopePaths(u *users.User) []string {
	out := []string{}
	for _, s := range h.fs.Sources() {
		for _, bs := range u.BackendScopes {
			if filepath.Clean(bs.Path) == s.Root {
				out = append(out, path.Join(s.Mount, "/"+bs.Scope))
			}
		}
	}
	slices.Sort(out)
	return out
}

// scopesFrom turns the folder a user may see into the scopes of the login
// system, one per source. A folder like /mnt that only leads to sources
// stands for all of them.
func (h handlers) scopesFrom(scope *string, role string) ([]users.BackendScope, error) {
	missing := v1.Errorf(http.StatusBadRequest, "pick the folder the user may see")
	if scope == nil || strings.TrimSpace(*scope) == "" {
		return nil, missing
	}
	notThere := v1.Errorf(http.StatusBadRequest, "this folder does not exist")
	p, err := ccfs.Clean(strings.TrimSpace(*scope))
	if err != nil || strings.Contains(p, ccfs.ArchiveSep) {
		return nil, notThere
	}
	loc, err := h.fs.Resolve(p)
	if err != nil {
		return nil, notThere
	}
	perms := rolePerms(role)
	if !loc.Virtual() {
		if loc.Source.AdminOnly && role != roleAdmin {
			return nil, v1.Errorf(http.StatusBadRequest, "only admins may see the single disks and the cache")
		}
		if fi, err := os.Stat(loc.Real); err != nil || !fi.IsDir() {
			return nil, notThere
		}
		return []users.BackendScope{{Path: loc.Source.Root, Scope: loc.Index, Permissions: perms}}, nil
	}
	var out []users.BackendScope
	for _, s := range h.fs.Sources() {
		if (p == "/" || s.Mount == p || strings.HasPrefix(s.Mount, p+"/")) && (!s.AdminOnly || role == roleAdmin) {
			out = append(out, users.BackendScope{Path: s.Root, Scope: "/", Permissions: perms})
		}
	}
	if len(out) == 0 {
		return nil, missing
	}
	return out, nil
}

// profile is what the app keeps about an account next to the login system.
type profile struct {
	name       string
	passwordAt int64
	seenAt     int64
}

func (h handlers) profiles(ctx context.Context) (map[string]profile, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT user_id, name, password_at, seen_at FROM account_profiles`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]profile{}
	for rows.Next() {
		var id string
		var p profile
		if err := rows.Scan(&id, &p.name, &p.passwordAt, &p.seenAt); err != nil {
			return nil, err
		}
		out[id] = p
	}
	return out, rows.Err()
}

func (h handlers) profileOf(ctx context.Context, id string) (profile, error) {
	var p profile
	err := h.db.QueryRowContext(ctx, `SELECT name, password_at, seen_at FROM account_profiles WHERE user_id = ?`, id).Scan(&p.name, &p.passwordAt, &p.seenAt)
	if errors.Is(err, sql.ErrNoRows) {
		return profile{}, nil
	}
	return p, err
}

func setName(ctx context.Context, db *sql.DB, id, name string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO account_profiles (user_id, name) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET name = excluded.name`, id, name)
	return err
}

func passwordChanged(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `INSERT INTO account_profiles (user_id, password_at) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET password_at = excluded.password_at`, id, time.Now().Unix())
	return err
}

func seenNow(ctx context.Context, db *sql.DB, id string, at int64) error {
	_, err := db.ExecContext(ctx, `INSERT INTO account_profiles (user_id, seen_at) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET seen_at = excluded.seen_at`, id, at)
	return err
}

func unixTime(s int64) *time.Time {
	if s == 0 {
		return nil
	}
	t := time.Unix(s, 0).UTC()
	return &t
}

func (h handlers) account(u *users.User, p profile) Account {
	full := p.name
	if full == "" {
		full = u.Username
	}
	groups := state.GetUserGroups(u.Username)
	slices.Sort(groups)
	return Account{
		ID:          idOf(u),
		Name:        u.Username,
		Full:        full,
		Role:        roleOf(u),
		Scope:       commonFolder(h.scopePaths(u)),
		Groups:      groups,
		LoginMethod: string(u.LoginMethod),
		Twofa:       u.TOTPSecret != "",
		Passkeys:    len(u.PasskeyCredentials),
		Last:        unixTime(p.seenAt),
		PwChanged:   unixTime(p.passwordAt),
	}
}

// commonFolder is the deepest folder that holds every path, so the scopes
// of a user who sees several sources read as the one folder they came from.
func commonFolder(paths []string) string {
	if len(paths) == 0 {
		return ""
	}
	out := paths[0]
	for _, p := range paths[1:] {
		for out != "/" && p != out && !strings.HasPrefix(p, out+"/") {
			out = path.Dir(out)
		}
	}
	return out
}

// checkPassword applies the app's minimum and the server's policy.
func checkPassword(pw string) error {
	if len(pw) < minPassword {
		return v1.Errorf(http.StatusBadRequest, "the password needs at least 12 characters")
	}
	if err := settings.ValidatePasswordPolicy(pw); err != nil {
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	return nil
}
