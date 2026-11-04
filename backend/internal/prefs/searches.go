package prefs

import (
	"encoding/json"
	"net/http"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

const (
	maxSearches   = 200
	maxQueryBytes = 16 << 10
)

var searchTable = named{table: "saved_searches", column: "query", limit: maxSearches}

// SavedSearch is a search kept under a name. Query is the search as the
// search window sends it, stored as given so the search can grow fields
// without a change here.
type SavedSearch struct {
	ID int64 `json:"id"`
	SearchInput
}

// SearchInput is a saved search as the client sends it.
type SearchInput struct {
	Name  string          `json:"name"`
	Query json.RawMessage `json:"query"`
}

func (in *SearchInput) validate() error {
	name, err := cleanName(in.Name, "a saved search")
	if err != nil {
		return err
	}
	in.Name = name
	if kindOf(in.Query) != kindObject {
		return v1.Errorf(http.StatusBadRequest, "the query must be an object")
	}
	if len(in.Query) > maxQueryBytes {
		return v1.Errorf(http.StatusRequestEntityTooLarge, "the query is too large")
	}
	var q struct {
		In string `json:"in"`
	}
	if err := json.Unmarshal(in.Query, &q); err != nil {
		return v1.Errorf(http.StatusBadRequest, "the query field in must be a path")
	}
	if q.In != "" {
		if _, err := ccfs.Clean(q.In); err != nil || strings.Contains(q.In, ccfs.ArchiveSep) {
			return v1.Errorf(http.StatusBadRequest, "invalid search folder "+q.In)
		}
	}
	return nil
}

func (h handlers) listSearches(w http.ResponseWriter, r *http.Request) error {
	rows, err := searchTable.list(r.Context(), h.db, userID(r))
	if err != nil {
		return err
	}
	items := make([]SavedSearch, 0, len(rows))
	for _, row := range rows {
		items = append(items, SavedSearch{row.ID, SearchInput{Name: row.Name, Query: row.Data}})
	}
	return v1.WriteJSON(w, r, http.StatusOK, list[SavedSearch]{items})
}

func readSearch(r *http.Request) (SearchInput, error) {
	var in SearchInput
	if err := v1.DecodeJSON(r, &in); err != nil {
		return in, err
	}
	return in, in.validate()
}

func (h handlers) createSearch(w http.ResponseWriter, r *http.Request) error {
	in, err := readSearch(r)
	if err != nil {
		return err
	}
	id, err := searchTable.create(r.Context(), h.db, userID(r), in.Name, in.Query)
	if err != nil {
		return err
	}
	h.changed(r, Change{Kind: "savedSearches"})
	return v1.WriteJSON(w, r, http.StatusCreated, SavedSearch{id, in})
}

func (h handlers) updateSearch(w http.ResponseWriter, r *http.Request) error {
	id, err := idParam(r)
	if err != nil {
		return err
	}
	in, err := readSearch(r)
	if err != nil {
		return err
	}
	if err := searchTable.update(r.Context(), h.db, userID(r), id, in.Name, in.Query); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "savedSearches"})
	return v1.WriteJSON(w, r, http.StatusOK, SavedSearch{id, in})
}

func (h handlers) deleteSearch(w http.ResponseWriter, r *http.Request) error {
	id, err := idParam(r)
	if err != nil {
		return err
	}
	if err := searchTable.delete(r.Context(), h.db, userID(r), id); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "savedSearches"})
	return noContent(w)
}
