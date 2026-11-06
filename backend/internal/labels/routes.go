// Package labels stores colour labels and tags on paths. They are shared:
// every user sees the labels of the files they may see.
//
// Labels hang on paths, so whoever renames, moves, copies or deletes files
// calls Moved, Copied or Removed afterwards to keep them on the right files.
package labels

import (
	"crypto/rand"
	"database/sql"
	_ "embed"
	"net/http"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

//go:embed openapi.yaml
var contract []byte

const (
	maxColours = 64
	maxTags    = 1000
	maxTagLen  = 64
	maxNameLen = 64
	maxPaths   = 10000
)

var hexColour = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// Routes registers the contract and its handlers.
func Routes(r *v1.Router) {
	r.Contract("labels", contract)
	d := r.Deps()
	h := handlers{db: d.DB, hub: d.Hub, fs: d.FS}
	r.Handle("GET /labels", h.overview)
	r.Handle("PUT /labels/colours", h.putColours)
	r.Handle("PUT /labels/tags", h.putTags)
	r.Handle("GET /labels/items", h.items)
	r.Handle("PUT /labels/items", h.setItems)
}

type handlers struct {
	db  *sql.DB
	hub *hub.Hub
	fs  *ccfs.Resolver
}

// Change is the payload of labels.changed. Kind is colours or tags for the
// lists, which reach everyone, or items for the labels of the entries of
// the folder Path, which reach the clients watching that folder.
type Change struct {
	Kind string `json:"kind"`
	Path string `json:"path,omitempty"`
}

// Colour is one colour label. An empty name stands for the name the app
// gives the colours it comes with, in the reader's language.
type Colour struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Hex   string `json:"hex"`
	Count int    `json:"count"`
}

// Tag is one tag with the number of paths carrying it.
type Tag struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Overview is the answer of GET /labels and of the list edits.
type Overview struct {
	Colours []Colour `json:"colours"`
	Tags    []Tag    `json:"tags"`
}

func (h handlers) overview(w http.ResponseWriter, r *http.Request) error {
	o, err := h.load(r)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, o)
}

func (h handlers) load(r *http.Request) (Overview, error) {
	ctx := r.Context()
	colourCount, tagCount, err := counts(ctx, h.db)
	if err != nil {
		return Overview{}, err
	}
	o := Overview{Colours: []Colour{}, Tags: []Tag{}}
	rows, err := h.db.QueryContext(ctx, `SELECT id, name, hex FROM label_colours ORDER BY pos`)
	if err != nil {
		return o, err
	}
	defer rows.Close()
	for rows.Next() {
		var c Colour
		if err := rows.Scan(&c.ID, &c.Name, &c.Hex); err != nil {
			return o, err
		}
		c.Count = colourCount[c.ID]
		o.Colours = append(o.Colours, c)
	}
	if err := rows.Err(); err != nil {
		return o, err
	}
	declared, err := declaredTags(r, h.db)
	if err != nil {
		return o, err
	}
	names := map[string]bool{}
	for _, t := range declared {
		names[t] = true
	}
	for t := range tagCount {
		names[t] = true
	}
	for t := range names {
		o.Tags = append(o.Tags, Tag{Name: t, Count: tagCount[t]})
	}
	coll := collate.New(language.German)
	slices.SortFunc(o.Tags, func(a, b Tag) int { return coll.CompareString(a.Name, b.Name) })
	return o, nil
}

func declaredTags(r *http.Request, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(r.Context(), `SELECT name FROM label_tags`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// cleanTag trims a tag and drops a leading #, which people type out of
// habit because the search finds tags with it.
func cleanTag(t string) (string, error) {
	t = strings.TrimPrefix(strings.TrimSpace(t), "#")
	if t == "" || utf8.RuneCountInString(t) > maxTagLen || strings.ContainsFunc(t, unicode.IsControl) {
		return "", v1.Coded(http.StatusBadRequest, "tagLength", "a tag needs between 1 and 64 characters")
	}
	return t, nil
}

type coloursRequest struct {
	Colours []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Hex  string `json:"hex"`
	} `json:"colours"`
}

// putColours replaces the colour list. Colours left out are taken off
// every path, colours without an id are new.
func (h handlers) putColours(w http.ResponseWriter, r *http.Request) error {
	var req coloursRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.Colours) > maxColours {
		return v1.Coded(http.StatusBadRequest, "tooManyColours", "too many colours")
	}
	ctx := r.Context()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	known := map[string]bool{}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM label_colours`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		known[id] = true
	}
	rows.Close()
	if _, err := tx.ExecContext(ctx, `DELETE FROM label_colours`); err != nil {
		return err
	}
	kept := map[string]bool{}
	for i, c := range req.Colours {
		name := strings.TrimSpace(c.Name)
		if utf8.RuneCountInString(name) > maxNameLen {
			return v1.Coded(http.StatusBadRequest, "colourNameLength", "a colour name has at most 64 characters")
		}
		if !hexColour.MatchString(c.Hex) {
			return v1.Errorf(http.StatusBadRequest, "invalid colour value "+c.Hex)
		}
		id := c.ID
		switch {
		case id == "":
			id = "c" + strings.ToLower(rand.Text()[:10])
		case !known[id] || kept[id]:
			return v1.Errorf(http.StatusBadRequest, "unknown colour id "+id)
		}
		kept[id] = true
		if _, err := tx.ExecContext(ctx, `INSERT INTO label_colours (id, name, hex, pos) VALUES (?, ?, ?, ?)`, id, name, strings.ToLower(c.Hex), i); err != nil {
			return err
		}
	}
	var dropped []any
	for id := range known {
		if !kept[id] {
			dropped = append(dropped, id)
		}
	}
	if len(dropped) > 0 {
		in := `(?` + strings.Repeat(",?", len(dropped)-1) + `)`
		if _, err := tx.ExecContext(ctx, `UPDATE label_items SET colour = NULL WHERE colour IN `+in, dropped...); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM label_items WHERE colour IS NULL AND NOT EXISTS (
			SELECT 1 FROM label_item_tags t WHERE t.path = label_items.path)`); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return h.listsChanged(w, r, "colours")
}

type tagsRequest struct {
	Tags []struct {
		From string `json:"from"`
		Name string `json:"name"`
	} `json:"tags"`
}

// putTags replaces the tag list. An entry with from renames that tag on
// every path, one without is a new tag, and tags left out are taken off
// every path.
func (h handlers) putTags(w http.ResponseWriter, r *http.Request) error {
	var req tagsRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.Tags) > maxTags {
		return v1.Coded(http.StatusBadRequest, "tooManyTags", "too many tags")
	}
	rename := map[string]string{}
	var names []string
	for _, t := range req.Tags {
		name, err := cleanTag(t.Name)
		if err != nil {
			return err
		}
		names = append(names, name)
		if t.From != "" {
			rename[t.From] = name
		}
	}
	ctx := r.Context()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	rows, err := tx.QueryContext(ctx, `SELECT path, tag FROM label_item_tags`)
	if err != nil {
		return err
	}
	type pair struct{ path, tag string }
	var all []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.path, &p.tag); err != nil {
			rows.Close()
			return err
		}
		all = append(all, p)
	}
	rows.Close()
	if _, err := tx.ExecContext(ctx, `DELETE FROM label_item_tags`); err != nil {
		return err
	}
	for _, p := range all {
		to, ok := rename[p.tag]
		if !ok {
			continue
		}
		// Two tags renamed to the same name become one.
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO label_item_tags (path, tag) VALUES (?, ?)`, p.path, to); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM label_items WHERE colour IS NULL AND NOT EXISTS (
		SELECT 1 FROM label_item_tags t WHERE t.path = label_items.path)`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM label_tags`); err != nil {
		return err
	}
	for _, n := range names {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO label_tags (name) VALUES (?)`, n); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return h.listsChanged(w, r, "tags")
}

// listsChanged answers a list edit with the new lists and tells everyone,
// since items in any folder may have changed with it.
func (h handlers) listsChanged(w http.ResponseWriter, r *http.Request, kind string) error {
	_, _ = h.hub.PublishJSON(EventChanged, "", Change{Kind: kind})
	o, err := h.load(r)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, o)
}

// Items is the answer of GET /labels/items. The keys are the paths of the
// labelled entries relative to Path.
type Items struct {
	Path  string           `json:"path"`
	Items map[string]Label `json:"items"`
}

func (h handlers) items(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	q := r.URL.Query()
	dir, err := h.fs.Check(u, q.Get("path"), ccfs.View)
	if err != nil {
		return err
	}
	deep := q.Get("deep") == "true"
	var found map[string]Label
	if deep {
		lo, hi := subtree(dir.Path)
		found, err = load(r.Context(), h.db, `i.path > ? AND i.path < ?`, lo, hi)
	} else {
		found, err = load(r.Context(), h.db, `i.dir = ?`, dir.Path)
	}
	if err != nil {
		return err
	}
	out := Items{Path: dir.Path, Items: map[string]Label{}}
	prefix, _ := subtree(dir.Path)
	for p, l := range found {
		rel := strings.TrimPrefix(p, prefix)
		if deep {
			if _, err := h.fs.Check(u, p, ccfs.View); err != nil {
				continue
			}
		} else if !h.fs.Visible(u, dir, rel) {
			continue
		}
		out.Items[rel] = l
	}
	return v1.WriteJSON(w, r, http.StatusOK, out)
}

type itemsRequest struct {
	Paths  []string  `json:"paths"`
	Colour *string   `json:"colour"`
	Tags   *[]string `json:"tags"`
	Add    []string  `json:"add"`
	Remove []string  `json:"remove"`
}

// setItems changes the labels of several paths in one go. Colour sets or,
// when empty, clears the colour. Tags replaces the tags, Add and Remove
// adjust them. Fields left out stay as they are.
func (h handlers) setItems(w http.ResponseWriter, r *http.Request) error {
	var req itemsRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	if len(req.Paths) == 0 || len(req.Paths) > maxPaths {
		return v1.Errorf(http.StatusBadRequest, "a request takes between 1 and 10000 paths")
	}
	cleanTags := func(in []string) ([]string, error) {
		out := []string{}
		for _, t := range in {
			c, err := cleanTag(t)
			if err != nil {
				return nil, err
			}
			out = append(out, c)
		}
		return out, nil
	}
	add, err := cleanTags(req.Add)
	if err != nil {
		return err
	}
	remove, err := cleanTags(req.Remove)
	if err != nil {
		return err
	}
	var replace []string
	if req.Tags != nil {
		if replace, err = cleanTags(*req.Tags); err != nil {
			return err
		}
	}
	u := v1.UserFrom(r)
	paths := make([]string, 0, len(req.Paths))
	for _, p := range req.Paths {
		// Labels change what everyone sees on the file, so they need the
		// right to change it.
		loc, err := h.fs.Check(u, p, ccfs.Modify)
		if err != nil {
			return err
		}
		if _, err := os.Lstat(loc.Real); err != nil {
			return err
		}
		paths = append(paths, loc.Path)
	}
	slices.Sort(paths)
	paths = slices.Compact(paths)

	ctx := r.Context()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if req.Colour != nil && *req.Colour != "" {
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM label_colours WHERE id = ?`, *req.Colour).Scan(&n); err != nil {
			return err
		}
		if n == 0 {
			return v1.Errorf(http.StatusBadRequest, "unknown colour "+*req.Colour)
		}
	}
	before, err := loadPaths(ctx, tx, paths)
	if err != nil {
		return err
	}
	out := map[string]Label{}
	var folders []string
	for _, p := range paths {
		l := before[p]
		if req.Colour != nil {
			l.Colour = *req.Colour
		}
		if req.Tags != nil {
			l.Tags = slices.Clone(replace)
		}
		l.Tags = append(l.Tags, add...)
		l.Tags = slices.DeleteFunc(l.Tags, func(t string) bool { return slices.Contains(remove, t) })
		slices.Sort(l.Tags)
		l.Tags = slices.Compact(l.Tags)
		if err := put(ctx, tx, p, l); err != nil {
			return err
		}
		out[p] = l
		folders = append(folders, path.Dir(p))
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	publishItems(h.hub, folders)
	return v1.WriteJSON(w, r, http.StatusOK, map[string]any{"items": out})
}
