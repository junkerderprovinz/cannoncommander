package accounts

import (
	"context"
	"net/http"
	"regexp"
	"slices"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

var groupName = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

// Group is a named set of users that rules can be written for.
type Group struct {
	Name    string   `json:"name"`
	Note    string   `json:"note"`
	Members []string `json:"members"`
}

// GroupInput creates or changes a group. In a change, fields that are left
// out stay as they are.
type GroupInput struct {
	Name    *string  `json:"name"`
	Note    *string  `json:"note"`
	Members []string `json:"members"`
}

func (h handlers) notes(ctx context.Context) (map[string]string, error) {
	rows, err := h.db.QueryContext(ctx, `SELECT name, note FROM group_notes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, note string
		if err := rows.Scan(&name, &note); err != nil {
			return nil, err
		}
		out[name] = note
	}
	return out, rows.Err()
}

func (h handlers) setNote(ctx context.Context, name, note string) error {
	_, err := h.db.ExecContext(ctx, `INSERT INTO group_notes (name, note) VALUES (?, ?)
		ON CONFLICT (name) DO UPDATE SET note = excluded.note`, name, note)
	return err
}

func (h handlers) listGroups(w http.ResponseWriter, r *http.Request) error {
	notes, err := h.notes(r.Context())
	if err != nil {
		return err
	}
	items := []Group{}
	for name, members := range state.GetGroupMembers() {
		items = append(items, Group{Name: name, Note: notes[name], Members: members})
	}
	slices.SortFunc(items, func(a, b Group) int { return strings.Compare(a.Name, b.Name) })
	return v1.WriteJSON(w, r, http.StatusOK, map[string][]Group{"items": items})
}

func (h handlers) group(ctx context.Context, name string) (Group, error) {
	notes, err := h.notes(ctx)
	if err != nil {
		return Group{}, err
	}
	return Group{Name: name, Note: notes[name], Members: state.GetGroupMembers()[name]}, nil
}

func checkGroupName(name string) error {
	switch {
	case name == "":
		return v1.Errorf(http.StatusBadRequest, "give the group a name")
	case !groupName.MatchString(name):
		return v1.Errorf(http.StatusBadRequest, "only lowercase letters, digits, _ and - please")
	case state.GroupExists(name):
		return v1.Errorf(http.StatusConflict, "this group exists already")
	}
	return nil
}

// checkMembers makes sure every member is a user and returns them sorted.
func checkMembers(members []string) ([]string, error) {
	out := []string{}
	for _, m := range members {
		if _, err := state.GetUserByUsername(m); err != nil {
			return nil, v1.Errorf(http.StatusBadRequest, "there is no user "+m)
		}
		if !slices.Contains(out, m) {
			out = append(out, m)
		}
	}
	slices.Sort(out)
	return out, nil
}

func (h handlers) createGroup(w http.ResponseWriter, r *http.Request) error {
	var in GroupInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	name := strings.TrimSpace(deref(in.Name))
	if err := checkGroupName(name); err != nil {
		return err
	}
	members, err := checkMembers(in.Members)
	if err != nil {
		return err
	}
	if err := state.SetGroupMembers(name, members); err != nil {
		return err
	}
	if err := h.setNote(r.Context(), name, strings.TrimSpace(deref(in.Note))); err != nil {
		return err
	}
	g, err := h.group(r.Context(), name)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusCreated, g)
}

func (h handlers) updateGroup(w http.ResponseWriter, r *http.Request) error {
	var in GroupInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return err
	}
	name := r.PathValue("name")
	if !state.GroupExists(name) {
		return v1.Errorf(http.StatusNotFound, "there is no such group")
	}
	members := state.GetGroupMembers()[name]
	if in.Members != nil {
		var err error
		if members, err = checkMembers(in.Members); err != nil {
			return err
		}
	}
	newName := name
	if in.Name != nil && strings.TrimSpace(*in.Name) != name {
		newName = strings.TrimSpace(*in.Name)
		if err := checkGroupName(newName); err != nil {
			return err
		}
	}
	notes, err := h.notes(r.Context())
	if err != nil {
		return err
	}
	note := notes[name]
	if in.Note != nil {
		note = strings.TrimSpace(*in.Note)
	}
	if err := state.SetGroupMembers(newName, members); err != nil {
		return err
	}
	if newName != name {
		if err := h.dropGroup(r.Context(), name); err != nil {
			return err
		}
		if err := h.svc.rules.renameWho(r.Context(), "@"+name, "@"+newName); err != nil {
			return err
		}
	}
	if err := h.setNote(r.Context(), newName, note); err != nil {
		return err
	}
	g, err := h.group(r.Context(), newName)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, g)
}

// dropGroup removes a group from the login system and its note.
func (h handlers) dropGroup(ctx context.Context, name string) error {
	if err := state.DeleteGroup(name); err != nil {
		return err
	}
	_, err := h.db.ExecContext(ctx, `DELETE FROM group_notes WHERE name = ?`, name)
	return err
}

// deleteGroup removes the group and its rules. Its members stay users.
func (h handlers) deleteGroup(w http.ResponseWriter, r *http.Request) error {
	name := r.PathValue("name")
	if !state.GroupExists(name) {
		return v1.Errorf(http.StatusNotFound, "there is no such group")
	}
	if err := h.dropGroup(r.Context(), name); err != nil {
		return err
	}
	if err := h.svc.rules.dropWho(r.Context(), "@"+name); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
