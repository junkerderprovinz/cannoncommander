package accounts

import (
	"cmp"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

var loginName = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,31}$`)

func checkLoginName(name string) error {
	switch {
	case name == "":
		return v1.Errorf(http.StatusBadRequest, "the login name is missing")
	case !loginName.MatchString(name):
		return v1.Errorf(http.StatusBadRequest, "the login name needs lowercase letters, at least two characters")
	}
	return nil
}

func (h handlers) listUsers(w http.ResponseWriter, r *http.Request) error {
	all, err := state.GetAllUsers()
	if err != nil {
		return err
	}
	profiles, err := h.profiles(r.Context())
	if err != nil {
		return err
	}
	items := make([]Account, 0, len(all))
	for i := range all {
		items = append(items, h.account(&all[i], profiles[idOf(&all[i])]))
	}
	slices.SortFunc(items, func(a, b Account) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Full), strings.ToLower(b.Full)), strings.Compare(a.Name, b.Name))
	})
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Account{"items": items})
}

// target loads the user the path names, by id or by login name. A login
// name starts with a letter, so the two never mix up.
func target(r *http.Request) (users.User, error) {
	key := r.PathValue("id")
	var u users.User
	var err error
	if id, perr := strconv.ParseUint(key, 10, 64); perr == nil {
		u, err = state.GetUserByID(id)
	} else {
		u, err = state.GetUserByUsername(key)
	}
	if err != nil {
		return users.User{}, v1.Errorf(http.StatusNotFound, "there is no such user")
	}
	return u, nil
}

func (h handlers) getUser(w http.ResponseWriter, r *http.Request) error {
	u, err := target(r)
	if err != nil {
		return err
	}
	return h.writeAccount(w, r, http.StatusOK, &u)
}

func (h handlers) writeAccount(w http.ResponseWriter, r *http.Request, status int, u *users.User) error {
	p, err := h.profileOf(r.Context(), idOf(u))
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, status, h.account(u, p))
}

// UserInput creates or changes a user. In a change, fields that are left
// out stay as they are.
type UserInput struct {
	Name     *string  `json:"name"`
	Full     *string  `json:"full"`
	Role     *string  `json:"role"`
	Scope    *string  `json:"scope"`
	Groups   []string `json:"groups"`
	Password *string  `json:"password"`
}

func checkGroups(names []string) error {
	for _, g := range names {
		if !state.GroupExists(g) {
			return v1.Errorf(http.StatusBadRequest, "there is no group "+g)
		}
	}
	return nil
}

// setGroups makes the user a member of exactly these groups.
func setGroups(username string, want []string) error {
	have := state.GetUserGroups(username)
	for _, g := range have {
		if !slices.Contains(want, g) {
			if err := state.RemoveUserFromGroup(g, username); err != nil {
				return err
			}
		}
	}
	for _, g := range want {
		if !slices.Contains(have, g) {
			if err := state.AddUserToGroup(g, username); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h handlers) createUser(w http.ResponseWriter, r *http.Request) error {
	var in UserInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(deref(in.Name))
	if err := checkLoginName(name); err != nil {
		return err
	}
	if _, err := state.GetUserByUsername(name); err == nil {
		return v1.Errorf(http.StatusConflict, "this login name is taken")
	}
	role := cmp.Or(deref(in.Role), roleUser)
	if !validRole(role) {
		return v1.Errorf(http.StatusBadRequest, "the role has to be admin, user or readonly")
	}
	scopes, err := h.scopesFrom(in.Scope, role)
	if err != nil {
		return err
	}
	if err := checkGroups(in.Groups); err != nil {
		return err
	}
	if in.Password == nil {
		return v1.Errorf(http.StatusBadRequest, "the password is missing")
	}
	if err := checkPassword(*in.Password); err != nil {
		return err
	}
	perms := users.Permissions{Admin: role == roleAdmin, Share: mayShareAs(role)}
	u := users.User{FrontendUser: users.FrontendUser{
		Username:    name,
		LoginMethod: users.LoginMethodPassword,
		Permissions: users.MarkPermissionsConfigured(perms),
	}, BackendScopes: scopes}
	if err := state.CreateUser(&u, *in.Password); err != nil {
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	// The login system adds the default sources on create. The scopes
	// picked here are the ones that count.
	u.BackendScopes, u.Permissions = scopes, users.MarkPermissionsConfigured(perms)
	if err := state.UpdateUser(&u, "", "backendScopes", "permissions"); err != nil {
		if rmErr := state.DeleteUser(u.ID); rmErr != nil {
			logger.Errorf("remove half created user %s: %v", name, rmErr)
		}
		return v1.Errorf(http.StatusBadRequest, err.Error())
	}
	if err := setGroups(name, in.Groups); err != nil {
		return err
	}
	if err := setName(r.Context(), h.db, idOf(&u), strings.TrimSpace(deref(in.Full))); err != nil {
		return err
	}
	if err := passwordChanged(r.Context(), h.db, idOf(&u)); err != nil {
		return err
	}
	actor := v1.UserFrom(r)
	activity.RecordUserMutation(r, &activity.Actor{User: actor}, activitydb.EventUserCreate, &u, nil)
	if u, err = state.GetUserByID(u.ID); err != nil {
		return err
	}
	return h.writeAccount(w, r, http.StatusCreated, &u)
}

func (h handlers) updateUser(w http.ResponseWriter, r *http.Request) error {
	var in UserInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := target(r)
	if err != nil {
		return err
	}
	actor := v1.UserFrom(r)
	self := u.ID == actor.ID
	oldName := u.Username
	var fields []string

	if in.Name != nil && strings.TrimSpace(*in.Name) != u.Username {
		name := strings.TrimSpace(*in.Name)
		if self {
			return v1.Errorf(http.StatusBadRequest, "you cannot change your own login name")
		}
		if err := checkLoginName(name); err != nil {
			return err
		}
		if _, err := state.GetUserByUsername(name); err == nil {
			return v1.Errorf(http.StatusConflict, "this login name is taken")
		}
		u.Username = name
		fields = append(fields, "username")
	}

	role := roleOf(&u)
	if in.Role != nil && *in.Role != role {
		if !validRole(*in.Role) {
			return v1.Errorf(http.StatusBadRequest, "the role has to be admin, user or readonly")
		}
		if role == roleAdmin && self {
			return v1.Errorf(http.StatusConflict, "only another admin can take away your own admin role")
		}
		role = *in.Role
		u.Permissions.Admin, u.Permissions.Share = role == roleAdmin, mayShareAs(role)
		u.Permissions = users.MarkPermissionsConfigured(u.Permissions)
		fields = append(fields, "permissions")
	}

	if in.Scope != nil {
		scopes, err := h.scopesFrom(in.Scope, role)
		if err != nil {
			return err
		}
		u.BackendScopes = scopes
		fields = append(fields, "backendScopes")
	} else if in.Role != nil {
		u.BackendScopes = h.withRole(u.BackendScopes, role)
		if len(u.BackendScopes) == 0 {
			return v1.Errorf(http.StatusBadRequest, "pick the folder the user may see")
		}
		fields = append(fields, "backendScopes")
	}

	if in.Groups != nil {
		if err := checkGroups(in.Groups); err != nil {
			return err
		}
	}

	password := ""
	if in.Password != nil {
		if self {
			return v1.Errorf(http.StatusBadRequest, "change your own password under account and security")
		}
		if u.LoginMethod != users.LoginMethodPassword {
			return v1.Errorf(http.StatusConflict, "this user signs in through "+string(u.LoginMethod)+", the password is kept there")
		}
		if err := checkPassword(*in.Password); err != nil {
			return err
		}
		password = *in.Password
		fields = append(fields, "password")
	}

	if len(fields) > 0 {
		if err := state.UpdateUser(&u, password, fields...); err != nil {
			return v1.Errorf(http.StatusBadRequest, err.Error())
		}
	}
	if u.Username != oldName {
		if err := h.svc.rules.renameWho(r.Context(), oldName, u.Username); err != nil {
			return err
		}
	}
	if in.Groups != nil {
		if err := setGroups(u.Username, in.Groups); err != nil {
			return err
		}
	}
	if in.Full != nil {
		if err := setName(r.Context(), h.db, idOf(&u), strings.TrimSpace(*in.Full)); err != nil {
			return err
		}
	}
	if password != "" {
		// A new password set by an admin logs the user out everywhere.
		if _, err := login.EndAll(u.ID, ""); err != nil {
			return err
		}
		if err := passwordChanged(r.Context(), h.db, idOf(&u)); err != nil {
			return err
		}
	}
	changes := make([]activitydb.FieldChange, 0, len(fields))
	for _, f := range fields {
		changes = append(changes, activitydb.FieldChange{Field: f})
	}
	activity.RecordUserMutation(r, &activity.Actor{User: actor}, activitydb.EventUserUpdate, &u, changes)
	if u, err = state.GetUserByID(u.ID); err != nil {
		return err
	}
	return h.writeAccount(w, r, http.StatusOK, &u)
}

// withRole gives the scopes the rights of role. A user who is no admin any
// more loses the single disks and the cache.
func (h handlers) withRole(scopes []users.BackendScope, role string) []users.BackendScope {
	adminOnly := map[string]bool{}
	for _, s := range h.fs.Sources() {
		adminOnly[s.Root] = s.AdminOnly
	}
	out := make([]users.BackendScope, 0, len(scopes))
	for _, bs := range scopes {
		if adminOnly[filepath.Clean(bs.Path)] && role != roleAdmin {
			continue
		}
		bs.Permissions = rolePerms(role)
		out = append(out, bs)
	}
	return out
}

// mayShareAs keeps links from read only users, because a public link hands a
// file to anyone, which goes further than looking at it and downloading it.
func mayShareAs(role string) bool { return role != roleReadOnly }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
