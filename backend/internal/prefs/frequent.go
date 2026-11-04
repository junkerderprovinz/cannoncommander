package prefs

import (
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

const (
	maxFrequent    = 500
	defaultTopDirs = 8
	maxTopDirs     = 50
)

// Folder is an often visited folder with its rank.
type Folder struct {
	Path   string    `json:"path"`
	Visits int       `json:"visits"`
	Last   time.Time `json:"last"`
	Rank   float64   `json:"rank"`
}

// rank weighs the visits by how recent the last one was, the way zoxide
// does, so a folder used a lot last year gives way to this week's work.
func rank(visits int, last, now time.Time) float64 {
	age := now.Sub(last)
	w := 0.25
	switch {
	case age < time.Hour:
		w = 4
	case age < 24*time.Hour:
		w = 2
	case age < 7*24*time.Hour:
		w = 0.5
	}
	return float64(visits) * w
}

// topFolders returns the n best ranked folders of a user that still exist
// and that the user may open, leaving out skip.
func (h handlers) topFolders(r *http.Request, n int, skip string) ([]Folder, error) {
	rows, err := h.db.QueryContext(r.Context(), `SELECT path, visits, last_at FROM frequent_dirs WHERE user_id = ?`, userID(r))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	now := time.Now()
	var all []Folder
	for rows.Next() {
		var f Folder
		var last int64
		if err := rows.Scan(&f.Path, &f.Visits, &last); err != nil {
			return nil, err
		}
		f.Last = time.Unix(last, 0).UTC()
		f.Rank = rank(f.Visits, f.Last, now)
		all = append(all, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	slices.SortStableFunc(all, func(a, b Folder) int {
		if a.Rank != b.Rank {
			if a.Rank > b.Rank {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Path, b.Path)
	})
	u := v1.UserFrom(r)
	out := []Folder{}
	for _, f := range all {
		if len(out) == n {
			break
		}
		if f.Path == skip {
			continue
		}
		if loc, err := h.fs.Check(u, f.Path, ccfs.View); err != nil || !isDir(loc) {
			continue
		}
		out = append(out, f)
	}
	return out, nil
}

func isDir(loc ccfs.Location) bool {
	if loc.Virtual() {
		return true
	}
	fi, err := os.Stat(loc.Real)
	return err == nil && fi.IsDir()
}

func (h handlers) getFrequent(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	n := defaultTopDirs
	if s := q.Get("limit"); s != "" {
		v, err := strconv.Atoi(s)
		if err != nil || v < 1 || v > maxTopDirs {
			return v1.Errorf(http.StatusBadRequest, "limit must be between 1 and 50")
		}
		n = v
	}
	skip := q.Get("skip")
	if skip != "" {
		clean, err := ccfs.Clean(skip)
		if err != nil {
			return v1.Errorf(http.StatusBadRequest, "invalid skip path")
		}
		skip = clean
	}
	items, err := h.topFolders(r, n, skip)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, list[Folder]{items})
}

type visitRequest struct {
	Path string `json:"path"`
}

func (h handlers) visit(w http.ResponseWriter, r *http.Request) error {
	var req visitRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	loc, err := h.fs.Check(v1.UserFrom(r), req.Path, ccfs.View)
	if err != nil {
		return err
	}
	// A folder inside an archive is a stop on the way, not a place to come back to.
	if loc.InArchive() {
		return noContent(w)
	}
	if !loc.Virtual() {
		fi, err := os.Stat(loc.Real)
		if err != nil {
			return err
		}
		if !fi.IsDir() {
			return v1.Errorf(http.StatusBadRequest, "only folders are counted")
		}
	}
	ctx, uid, now := r.Context(), userID(r), time.Now().Unix()
	tx, err := h.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO frequent_dirs (user_id, path, visits, last_at) VALUES (?, ?, 1, ?)
		ON CONFLICT (user_id, path) DO UPDATE SET visits = visits + 1, last_at = excluded.last_at`, uid, loc.Path, now); err != nil {
		return err
	}
	// The history stays bounded: the folders not visited for longest go first.
	if _, err := tx.ExecContext(ctx, `DELETE FROM frequent_dirs WHERE user_id = ? AND path NOT IN (
		SELECT path FROM frequent_dirs WHERE user_id = ? ORDER BY last_at DESC, visits DESC LIMIT ?)`, uid, uid, maxFrequent); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return noContent(w)
}
