package accounts

import (
	"cmp"
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/activity"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	activitydb "github.com/junkerderprovinz/cannoncommander/backend/internal/database/activity"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// The Users tile manages the accounts of the sign-in through its Go API, as
// the admin of the request, and keeps what only CannonCommander knows next
// to them: scopes, groups and the name to show.

var loginName = regexp.MustCompile(`^[a-z][a-z0-9._-]{1,31}$`)

func checkLoginName(name string) error {
	switch {
	case name == "":
		return v1.Coded(http.StatusBadRequest, "loginNameMissing", "the login name is missing")
	case !loginName.MatchString(name):
		return v1.Coded(http.StatusBadRequest, "loginNameRule", "the login name needs lowercase letters, at least two characters")
	}
	return nil
}

var errNoSignIn = v1.Coded(http.StatusConflict, "noSignIn", "this server signs nobody in")

func signInOf() (*signin.Auth, error) {
	auth := SignIn()
	if auth == nil {
		return nil, errNoSignIn
	}
	return auth, nil
}

// fromSignIn turns what the sign-in refused into an answer of /api/v1 with
// the sign-in's code.
func fromSignIn(r *http.Request, err error) error {
	var no *signin.Refusal
	switch {
	case err == nil:
		return nil
	case errors.As(err, &no):
		return v1.Coded(no.Status, no.Code, no.Code)
	case errors.Is(err, signin.ErrNotFresh):
		return signInAgain{SignIn()}
	case errors.Is(err, signin.ErrNotFound):
		return errNoSuchUser
	}
	return err
}

var errNoSuchUser = v1.Coded(http.StatusNotFound, "noSuchUser", "there is no such user")

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
		a, err := h.adminView(r, &all[i], profiles[idOf(&all[i])])
		if err != nil {
			return err
		}
		items = append(items, a)
	}
	slices.SortFunc(items, func(a, b Account) int {
		return cmp.Or(strings.Compare(strings.ToLower(a.Full), strings.ToLower(b.Full)), strings.Compare(a.Name, b.Name))
	})
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Account{"items": items})
}

// adminView is the account u as an admin sees it, with its passkeys.
func (h handlers) adminView(r *http.Request, u *users.User, p profile) (Account, error) {
	auth, err := signInOf()
	if err != nil {
		return Account{}, err
	}
	acc, err := auth.Account(r.Context(), idOf(u))
	if err != nil && !errors.Is(err, signin.ErrNotFound) {
		return Account{}, err
	}
	a := h.account(u, p, acc)
	if err == nil {
		keys, err := auth.Passkeys(r, acc.ID)
		if err != nil {
			return Account{}, fromSignIn(r, err)
		}
		a.Passkeys = len(keys)
	}
	return a, nil
}

// target loads the user the path names, by id or by login name.
func target(r *http.Request) (users.User, error) {
	u, err := userByKey(r.PathValue("id"))
	if err != nil {
		return users.User{}, errNoSuchUser
	}
	return u, nil
}

func (h handlers) getUser(w http.ResponseWriter, r *http.Request) error {
	u, err := target(r)
	if err != nil {
		return err
	}
	return h.writeAccount(w, r, http.StatusOK, u.ID, "")
}

// savedAccount is an account as a create or change answers it. Short says
// the password this request set is under the length the page points out.
type savedAccount struct {
	Account
	Short bool `json:"short,omitempty"`
}

// writeAccount answers with the account id. password is the one the request
// set, or "".
func (h handlers) writeAccount(w http.ResponseWriter, r *http.Request, status int, id uint64, password string) error {
	u, err := state.GetUserByID(id)
	if err != nil {
		return errNoSuchUser
	}
	p, err := h.profileOf(r.Context(), idOf(&u))
	if err != nil {
		return err
	}
	a, err := h.adminView(r, &u, p)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, status, savedAccount{a, settings.ShortPassword(password)})
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
	Blocked  *bool    `json:"blocked"`
	// Revoke ends every session, API token and passkey of the user. Only a
	// change looks at it.
	Revoke bool `json:"revoke"`
}

func checkGroups(names []string) error {
	for _, g := range names {
		if !state.GroupExists(g) {
			return v1.Coded(http.StatusBadRequest, "noGroupNamed", "there is no group "+g).With("name", g)
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
	auth, err := signInOf()
	if err != nil {
		return err
	}
	var in UserInput
	if err = v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(deref(in.Name))
	if err = checkLoginName(name); err != nil {
		return err
	}
	if _, err := state.GetUserByUsername(name); err == nil {
		return v1.Coded(http.StatusConflict, "loginNameTaken", "this login name is taken")
	}
	role := cmp.Or(deref(in.Role), roleUser)
	if !validRole(role) {
		return v1.Errorf(http.StatusBadRequest, "the role has to be admin, user or readonly")
	}
	scopes, err := h.scopesFrom(in.Scope, role)
	if err != nil {
		return err
	}
	if err = checkGroups(in.Groups); err != nil {
		return err
	}
	if in.Password == nil {
		return v1.Coded(http.StatusBadRequest, "passwordMissing", "the password is missing")
	}
	if err = checkPassword(*in.Password); err != nil {
		return err
	}
	ctx := context.WithValue(r.Context(), newUserKey{}, &newUser{scopes: scopes})
	acc, err := auth.CreateAccount(r.WithContext(ctx), name, *in.Password, role)
	if err != nil {
		h.undoCreate(r.Context(), name)
		return fromSignIn(r, err)
	}
	u, err := userOf(signin.Visitor{Account: acc.ID, Role: acc.Role})
	if err != nil {
		return err
	}
	if err = setGroups(name, in.Groups); err != nil {
		return err
	}
	if err = setName(r.Context(), h.db, acc.ID, strings.TrimSpace(deref(in.Full))); err != nil {
		return err
	}
	if err = passwordChanged(r.Context(), h.db, acc.ID); err != nil {
		return err
	}
	activity.RecordUserMutation(r, &activity.Actor{User: v1.UserFrom(r)}, activitydb.EventUserCreate, u, nil)
	return h.writeAccount(w, r, http.StatusCreated, u.ID, *in.Password)
}

// undoCreate takes back the row a create wrote when the sign-in could not
// write its account.
func (h handlers) undoCreate(ctx context.Context, name string) {
	u, err := state.GetUserByUsername(name)
	if err != nil {
		return
	}
	if _, err := SignIn().Account(ctx, idOf(&u)); !errors.Is(err, signin.ErrNotFound) {
		return
	}
	if state.DeleteUser(u.ID) == nil {
		_ = forgetRows(ctx, h, u.ID)
	}
}

func (h handlers) updateUser(w http.ResponseWriter, r *http.Request) error {
	auth, err := signInOf()
	if err != nil {
		return err
	}
	var in UserInput
	if err = v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	u, err := target(r)
	if err != nil {
		return err
	}
	id := idOf(&u)
	acc, err := auth.Account(r.Context(), id)
	if err != nil {
		return fromSignIn(r, err)
	}
	actor := v1.UserFrom(r)
	self := isSelf(u, actor)
	var fields []string

	if in.Revoke && self {
		return v1.Coded(http.StatusBadRequest, "ownSessionsElsewhere", "end your own sessions, tokens and passkeys under account and security")
	}
	if in.Blocked != nil && self {
		return v1.Coded(http.StatusBadRequest, "ownBlock", "you cannot block your own account")
	}
	name := u.Username
	if in.Name != nil && strings.TrimSpace(*in.Name) != u.Username {
		name = strings.TrimSpace(*in.Name)
		if self {
			return v1.Coded(http.StatusBadRequest, "ownLoginName", "you cannot change your own login name")
		}
		if err = checkLoginName(name); err != nil {
			return err
		}
		if _, err := state.GetUserByUsername(name); err == nil {
			return v1.Coded(http.StatusConflict, "loginNameTaken", "this login name is taken")
		}
	}
	role := acc.Role
	if in.Role != nil && *in.Role != role {
		if !validRole(*in.Role) {
			return v1.Errorf(http.StatusBadRequest, "the role has to be admin, user or readonly")
		}
		if role == roleAdmin && self {
			return v1.Coded(http.StatusConflict, "ownAdminRole", "only another admin can take away your own admin role")
		}
		if in.Scope == nil && len(h.withRole(u.BackendScopes, *in.Role)) == 0 {
			return v1.Coded(http.StatusBadRequest, "pickUserFolder", "pick the folder the user may see")
		}
	}
	var scopes []users.BackendScope
	if in.Scope != nil {
		if scopes, err = h.scopesFrom(in.Scope, cmp.Or(deref(in.Role), role)); err != nil {
			return err
		}
	}
	if in.Groups != nil {
		if err = checkGroups(in.Groups); err != nil {
			return err
		}
	}
	if in.Password != nil {
		if self {
			return v1.Coded(http.StatusBadRequest, "ownPasswordElsewhere", "change your own password under account and security")
		}
		if !acc.Password {
			return v1.Coded(http.StatusConflict, "userPasswordElsewhere", "this user signs in through a sign-in service, the password is kept there")
		}
		if err = checkPassword(*in.Password); err != nil {
			return err
		}
	}

	if name != u.Username {
		if err = auth.RenameAccount(r, id, name); err != nil {
			return fromSignIn(r, err)
		}
		fields = append(fields, "username")
	}
	if in.Role != nil && *in.Role != role {
		if err = auth.SetRole(r, id, *in.Role); err != nil {
			return fromSignIn(r, err)
		}
		fields = append(fields, "permissions")
	}
	if scopes != nil {
		fresh, lookupErr := state.GetUserByID(u.ID)
		if lookupErr != nil {
			return lookupErr
		}
		fresh.BackendScopes = scopes
		if err = state.UpdateUser(&fresh, "", "backendScopes"); err != nil {
			return v1.Errorf(http.StatusBadRequest, err.Error())
		}
		fields = append(fields, "backendScopes")
	}
	if in.Blocked != nil && *in.Blocked != acc.Blocked {
		if err = auth.SetBlocked(r, id, *in.Blocked); err != nil {
			return fromSignIn(r, err)
		}
		fields = append(fields, "blocked")
	}
	if in.Groups != nil {
		if err = setGroups(name, in.Groups); err != nil {
			return err
		}
	}
	if in.Full != nil {
		if err = setName(r.Context(), h.db, id, strings.TrimSpace(*in.Full)); err != nil {
			return err
		}
	}
	if in.Password != nil {
		if err = auth.SetPassword(r, id, *in.Password); err != nil {
			return fromSignIn(r, err)
		}
		fields = append(fields, "password")
	}
	if in.Revoke {
		if err = auth.Revoke(r, id); err != nil {
			return fromSignIn(r, err)
		}
		fields = append(fields, "access")
	}
	changes := make([]activitydb.FieldChange, 0, len(fields))
	for _, f := range fields {
		changes = append(changes, activitydb.FieldChange{Field: f})
	}
	if changed, err := state.GetUserByID(u.ID); err == nil {
		activity.RecordUserMutation(r, &activity.Actor{User: actor}, activitydb.EventUserUpdate, &changed, changes)
	}
	return h.writeAccount(w, r, http.StatusOK, u.ID, deref(in.Password))
}

// deleteUser removes an account. The sign-in ends its ways in, and the
// account's shares, jobs, undo steps, rules, settings and keys go as soon
// as the sign-in tells, before its name is free again.
func (h handlers) deleteUser(w http.ResponseWriter, r *http.Request) error {
	auth, err := signInOf()
	if err != nil {
		return err
	}
	u, err := target(r)
	if err != nil {
		return err
	}
	actor := v1.UserFrom(r)
	if isSelf(u, actor) {
		return v1.Coded(http.StatusForbidden, "deleteSelf", "you cannot delete your own account")
	}
	if err := auth.RemoveAccount(r, idOf(&u)); err != nil {
		return fromSignIn(r, err)
	}
	activity.RecordUserMutation(r, &activity.Actor{User: actor}, activitydb.EventUserDelete, &u, nil)
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// Token is an API token of a user, without its secret.
type Token struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Rights  string     `json:"rights"`
	Created time.Time  `json:"created"`
	Expires *time.Time `json:"expires"`
	Last    *time.Time `json:"last"`
}

// Passkey is a passkey of a user. Origin is the host it works for.
type Passkey struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Origin  string     `json:"origin"`
	Created time.Time  `json:"created"`
	Last    *time.Time `json:"last"`
}

// Session is one open sign-in of a user.
type Session struct {
	ID        string    `json:"id"`
	Created   time.Time `json:"created"`
	Seen      time.Time `json:"seen"`
	UserAgent string    `json:"userAgent"`
	IP        string    `json:"ip"`
	Current   bool      `json:"current"`
}

func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	t = t.UTC()
	return &t
}

func rightsOf(scopes []string) string {
	if slices.Contains(scopes, scopeAll) {
		return "rw"
	}
	return "r"
}

func (h handlers) userTokens(w http.ResponseWriter, r *http.Request) error {
	auth, u, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	list, err := auth.Tokens(r, idOf(&u))
	if err != nil {
		return fromSignIn(r, err)
	}
	items := make([]Token, 0, len(list))
	for _, t := range list {
		items = append(items, Token{ID: t.ID, Name: t.Name, Rights: rightsOf(t.Scopes), Created: t.Created.UTC(), Expires: timeOrNil(t.Expires), Last: timeOrNil(t.Used)})
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Token{"items": items})
}

func (h handlers) revokeUserToken(w http.ResponseWriter, r *http.Request) error {
	auth, u, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	if err := auth.RevokeToken(r, idOf(&u), r.PathValue("token")); err != nil {
		return fromSignIn(r, err)
	}
	recordForUser(r, activitydb.EventTokenDelete, activitydb.Details{TargetUsername: u.Username})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) userPasskeys(w http.ResponseWriter, r *http.Request) error {
	auth, u, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	list, err := auth.Passkeys(r, idOf(&u))
	if err != nil {
		return fromSignIn(r, err)
	}
	items := make([]Passkey, 0, len(list))
	for _, p := range list {
		items = append(items, Passkey{ID: p.ID, Name: p.Name, Origin: p.RPID, Created: p.Created.UTC(), Last: timeOrNil(p.Used)})
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Passkey{"items": items})
}

func (h handlers) removeUserPasskey(w http.ResponseWriter, r *http.Request) error {
	auth, u, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	if err := auth.RemovePasskey(r, idOf(&u), r.PathValue("passkey")); err != nil {
		return fromSignIn(r, err)
	}
	recordForUser(r, activitydb.EventPasskeyDelete, activitydb.Details{TargetUsername: u.Username})
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) userSessions(w http.ResponseWriter, r *http.Request) error {
	auth, u, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	list, err := auth.Sessions(r, idOf(&u))
	if err != nil {
		return fromSignIn(r, err)
	}
	items := make([]Session, 0, len(list))
	for _, s := range list {
		items = append(items, Session{ID: s.ID, Created: s.Created.UTC(), Seen: s.Seen.UTC(), UserAgent: s.Agent, IP: s.From, Current: s.Current})
	}
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Session{"items": items})
}

func (h handlers) endUserSession(w http.ResponseWriter, r *http.Request) error {
	auth, u, err := h.adminTarget(r)
	if err != nil {
		return err
	}
	if err := auth.EndSession(r, idOf(&u), r.PathValue("session")); err != nil {
		return fromSignIn(r, err)
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h handlers) adminTarget(r *http.Request) (*signin.Auth, users.User, error) {
	auth, err := signInOf()
	if err != nil {
		return nil, users.User{}, err
	}
	u, err := target(r)
	return auth, u, err
}

func recordForUser(r *http.Request, event activitydb.EventType, details activitydb.Details) {
	activity.RecordUser(r, &activity.Actor{User: v1.UserFrom(r)}, activitydb.Entry{EventType: event, Details: details})
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
// isSelf reports whether actor changes its own account. While the server is
// open the visitor stands in for everyone, not for the account it acts as.
func isSelf(u users.User, actor *users.User) bool {
	return u.ID == actor.ID && !state.OpenAccess()
}

func mayShareAs(role string) bool { return role != roleReadOnly }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
