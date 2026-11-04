package prefs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

const (
	maxBookmarks = 200
	maxPresets   = 100
	maxNameLen   = 200
)

// Bookmark is a pinned folder in the sidebar.
type Bookmark struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	Glyph string `json:"g,omitempty"`
}

// starterBookmarks greet a user who never pinned anything. Only the ones
// that exist and that the user may open are offered.
var starterBookmarks = []Bookmark{
	{Name: "Downloads", Path: "/mnt/user/downloads", Glyph: "IconDownload"},
	{Name: "appdata", Path: "/mnt/user/appdata", Glyph: "IconConfig"},
	{Name: "Filme", Path: "/mnt/user/filme", Glyph: "IconFileVideo"},
	{Name: "Fotos", Path: "/mnt/user/fotos", Glyph: "IconFileImage"},
	{Name: "Backups", Path: "/mnt/user/backups", Glyph: "IconBackupNow"},
}

// RenamePreset is a saved pattern of the multi rename window.
type RenamePreset struct {
	Name    string        `json:"name"`
	Options RenameOptions `json:"options"`
}

// RenameOptions are the fields of the multi rename window a preset fills.
type RenameOptions struct {
	Name  string `json:"name"`
	Ext   string `json:"ext"`
	Find  string `json:"find"`
	Repl  string `json:"repl"`
	Regex bool   `json:"regex"`
	Case  string `json:"case"`
}

var renameCases = map[string]bool{"keep": true, "lower": true, "upper": true, "title": true}

type list[T any] struct {
	Items []T `json:"items"`
}

// loadList reads a per user list document. found is false when the user
// never saved one.
func loadList[T any](ctx context.Context, db *sql.DB, table, userID string) (items []T, found bool, err error) {
	var data string
	err = db.QueryRowContext(ctx, `SELECT items FROM `+table+` WHERE user_id = ?`, userID).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return []T{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	items = []T{}
	return items, true, json.Unmarshal([]byte(data), &items)
}

func saveList[T any](ctx context.Context, db *sql.DB, table, userID string, items []T) error {
	data, err := json.Marshal(items)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `INSERT INTO `+table+` (user_id, items, updated_at) VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET items = excluded.items, updated_at = excluded.updated_at`, userID, string(data), time.Now().Unix())
	return err
}

func cleanName(name, what string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > maxNameLen {
		return "", v1.Errorf(http.StatusBadRequest, what+" needs a name of at most 200 characters")
	}
	return name, nil
}

func (h handlers) getBookmarks(w http.ResponseWriter, r *http.Request) error {
	items, found, err := loadList[Bookmark](r.Context(), h.db, "bookmarks", userID(r))
	if err != nil {
		return err
	}
	if !found {
		items = h.starters(v1.UserFrom(r))
	}
	return v1.WriteJSON(w, r, http.StatusOK, list[Bookmark]{items})
}

func (h handlers) starters(u *users.User) []Bookmark {
	out := []Bookmark{}
	for _, b := range starterBookmarks {
		loc, err := h.fs.Check(u, b.Path, ccfs.View)
		if err != nil || loc.Virtual() {
			continue
		}
		if fi, err := os.Stat(loc.Real); err == nil && fi.IsDir() {
			out = append(out, b)
		}
	}
	return out
}

func (h handlers) putBookmarks(w http.ResponseWriter, r *http.Request) error {
	var body list[Bookmark]
	if err := v1.DecodeJSON(r, &body); err != nil {
		return err
	}
	if len(body.Items) > maxBookmarks {
		return v1.Errorf(http.StatusBadRequest, "too many bookmarks")
	}
	seen := map[string]bool{}
	items := make([]Bookmark, 0, len(body.Items))
	for _, b := range body.Items {
		name, err := cleanName(b.Name, "a bookmark")
		if err != nil {
			return err
		}
		p, err := ccfs.Clean(b.Path)
		if err != nil {
			return v1.Errorf(http.StatusBadRequest, "invalid bookmark path "+b.Path)
		}
		if strings.Contains(p, ccfs.ArchiveSep) {
			return v1.Errorf(http.StatusBadRequest, "a folder inside an archive cannot be a bookmark")
		}
		if seen[p] {
			return v1.Errorf(http.StatusConflict, "there is a bookmark for "+p+" already")
		}
		seen[p] = true
		if len(b.Glyph) > 64 {
			return v1.Errorf(http.StatusBadRequest, "invalid glyph name")
		}
		items = append(items, Bookmark{Name: name, Path: p, Glyph: b.Glyph})
	}
	if err := saveList(r.Context(), h.db, "bookmarks", userID(r), items); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "bookmarks"})
	return noContent(w)
}

func (h handlers) getPresets(w http.ResponseWriter, r *http.Request) error {
	items, _, err := loadList[RenamePreset](r.Context(), h.db, "rename_presets", userID(r))
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, list[RenamePreset]{items})
}

func (h handlers) putPresets(w http.ResponseWriter, r *http.Request) error {
	var body list[RenamePreset]
	if err := v1.DecodeJSON(r, &body); err != nil {
		return err
	}
	if len(body.Items) > maxPresets {
		return v1.Errorf(http.StatusBadRequest, "too many presets")
	}
	seen := map[string]bool{}
	items := make([]RenamePreset, 0, len(body.Items))
	for _, p := range body.Items {
		name, err := cleanName(p.Name, "a preset")
		if err != nil {
			return err
		}
		if seen[name] {
			return v1.Errorf(http.StatusConflict, "there is a preset named "+name+" already")
		}
		seen[name] = true
		if p.Options.Case == "" {
			p.Options.Case = "keep"
		}
		if !renameCases[p.Options.Case] {
			return v1.Errorf(http.StatusBadRequest, "unknown case "+p.Options.Case)
		}
		o := p.Options
		if len(o.Name)+len(o.Ext)+len(o.Find)+len(o.Repl) > 4096 {
			return v1.Errorf(http.StatusBadRequest, "the pattern of "+name+" is too long")
		}
		items = append(items, RenamePreset{Name: name, Options: o})
	}
	if err := saveList(r.Context(), h.db, "rename_presets", userID(r), items); err != nil {
		return err
	}
	h.changed(r, Change{Kind: "renamePresets"})
	return noContent(w)
}
