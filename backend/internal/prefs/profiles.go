package prefs

import (
	"encoding/json"
	"net/http"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

const (
	maxProfiles = 100
	maxTabs     = 50
	maxColumns  = 100
)

var profileTable = named{table: "profiles", column: "layout", limit: maxProfiles}

// Profile is a saved layout of both panels: their tabs, the active tab of
// each and the columns of the list.
type Profile struct {
	ID int64 `json:"id"`
	ProfileInput
}

// ProfileInput is a profile as the client sends it.
type ProfileInput struct {
	Name    string         `json:"name"`
	Dual    bool           `json:"dual"`
	Columns []string       `json:"columns"`
	Panels  []ProfilePanel `json:"panels"`
}

// ProfilePanel is one panel of a profile. Cur is the index of the active
// tab, Tabs are the folders its tabs show.
type ProfilePanel struct {
	Cur  int      `json:"cur"`
	Tabs []string `json:"tabs"`
}

type profileLayout struct {
	Dual    bool           `json:"dual"`
	Columns []string       `json:"columns"`
	Panels  []ProfilePanel `json:"panels"`
}

func (p *ProfileInput) validate() error {
	name, err := cleanName(p.Name, "a profile")
	if err != nil {
		return err
	}
	p.Name = name
	if len(p.Panels) < 1 || len(p.Panels) > 2 {
		return v1.Errorf(http.StatusBadRequest, "a profile has one or two panels")
	}
	if len(p.Columns) > maxColumns {
		return v1.Errorf(http.StatusBadRequest, "too many columns")
	}
	if p.Columns == nil {
		p.Columns = []string{}
	}
	for i := range p.Panels {
		panel := &p.Panels[i]
		if len(panel.Tabs) < 1 || len(panel.Tabs) > maxTabs {
			return v1.Errorf(http.StatusBadRequest, "a panel has between 1 and 50 tabs")
		}
		if panel.Cur < 0 || panel.Cur >= len(panel.Tabs) {
			return v1.Errorf(http.StatusBadRequest, "the active tab is not one of the tabs")
		}
		for j, t := range panel.Tabs {
			clean, err := ccfs.Clean(t)
			if err != nil {
				return v1.Errorf(http.StatusBadRequest, "invalid tab path "+t)
			}
			panel.Tabs[j] = clean
		}
	}
	return nil
}

func (h handlers) listProfiles(w http.ResponseWriter, r *http.Request) error {
	rows, err := profileTable.list(r.Context(), h.db, userID(r))
	if err != nil {
		return err
	}
	items := make([]Profile, 0, len(rows))
	for _, row := range rows {
		var l profileLayout
		if err := json.Unmarshal(row.Data, &l); err != nil {
			return err
		}
		items = append(items, Profile{row.ID, ProfileInput{Name: row.Name, Dual: l.Dual, Columns: l.Columns, Panels: l.Panels}})
	}
	return v1.WriteJSON(w, r, http.StatusOK, list[Profile]{items})
}

func readProfile(r *http.Request) (ProfileInput, error) {
	var in ProfileInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return in, err
	}
	return in, in.validate()
}

func (h handlers) createProfile(w http.ResponseWriter, r *http.Request) error {
	in, err := readProfile(r)
	if err != nil {
		return err
	}
	id, err := profileTable.create(r.Context(), h.db, userID(r), in.Name, profileLayout{in.Dual, in.Columns, in.Panels})
	if err != nil {
		return err
	}
	h.changed(r, Change{Kind: "profiles"})
	return v1.WriteJSON(w, r, http.StatusCreated, Profile{id, in})
}

func (h handlers) updateProfile(w http.ResponseWriter, r *http.Request) error {
	id, err := idParam(r)
	if err != nil {
		return err
	}
	in, err := readProfile(r)
	if err != nil {
		return err
	}
	if err := profileTable.update(r.Context(), h.db, userID(r), id, in.Name, profileLayout{in.Dual, in.Columns, in.Panels}); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "profiles"})
	return v1.WriteJSON(w, r, http.StatusOK, Profile{id, in})
}

func (h handlers) deleteProfile(w http.ResponseWriter, r *http.Request) error {
	id, err := idParam(r)
	if err != nil {
		return err
	}
	if err := profileTable.delete(r.Context(), h.db, userID(r), id); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "profiles"})
	return noContent(w)
}
