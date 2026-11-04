package labels

import (
	"context"
	"database/sql"
	"path"
	"slices"
	"strings"
	"time"
)

// Label is the colour and the tags of one path. A path without either has
// no label at all.
type Label struct {
	Colour string   `json:"colour,omitempty"`
	Tags   []string `json:"tags,omitempty"`
}

func (l Label) empty() bool { return l.Colour == "" && len(l.Tags) == 0 }

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// subtree returns the bounds of the paths strictly below p. Every path
// below p starts with p + "/", and "0" is the byte right after "/".
func subtree(p string) (lo, hi string) {
	if p != "/" {
		p += "/"
	}
	return p, p[:len(p)-1] + "0"
}

// load reads the labels matching where, which is a condition on the
// columns of label_items aliased as i.
func load(ctx context.Context, q querier, where string, args ...any) (map[string]Label, error) {
	out := map[string]Label{}
	rows, err := q.QueryContext(ctx, `SELECT i.path, COALESCE(i.colour, '') FROM label_items i WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p, c string
		if err := rows.Scan(&p, &c); err != nil {
			return nil, err
		}
		out[p] = Label{Colour: c}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows, err = q.QueryContext(ctx, `SELECT t.path, t.tag FROM label_item_tags t JOIN label_items i ON i.path = t.path WHERE `+where+` ORDER BY t.path, t.tag`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var p, tag string
		if err := rows.Scan(&p, &tag); err != nil {
			return nil, err
		}
		l := out[p]
		l.Tags = append(l.Tags, tag)
		out[p] = l
	}
	return out, rows.Err()
}

func loadTree(ctx context.Context, q querier, p string) (map[string]Label, error) {
	lo, hi := subtree(p)
	return load(ctx, q, `i.path = ? OR (i.path > ? AND i.path < ?)`, p, lo, hi)
}

func loadPaths(ctx context.Context, q querier, paths []string) (map[string]Label, error) {
	out := map[string]Label{}
	for chunk := range slices.Chunk(paths, 500) {
		got, err := load(ctx, q, `i.path IN (?`+strings.Repeat(",?", len(chunk)-1)+`)`, anys(chunk)...)
		if err != nil {
			return nil, err
		}
		for p, l := range got {
			out[p] = l
		}
	}
	return out, nil
}

func anys(s []string) []any {
	out := make([]any, len(s))
	for i, v := range s {
		out[i] = v
	}
	return out
}

func dropTree(ctx context.Context, q querier, p string) (int64, error) {
	lo, hi := subtree(p)
	if _, err := q.ExecContext(ctx, `DELETE FROM label_item_tags WHERE path = ? OR (path > ? AND path < ?)`, p, lo, hi); err != nil {
		return 0, err
	}
	res, err := q.ExecContext(ctx, `DELETE FROM label_items WHERE path = ? OR (path > ? AND path < ?)`, p, lo, hi)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// put stores l for p, replacing what was there. An empty label removes
// the row, so only labelled paths take up space.
func put(ctx context.Context, q querier, p string, l Label) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM label_item_tags WHERE path = ?`, p); err != nil {
		return err
	}
	if l.empty() {
		_, err := q.ExecContext(ctx, `DELETE FROM label_items WHERE path = ?`, p)
		return err
	}
	var colour any
	if l.Colour != "" {
		colour = l.Colour
	}
	if _, err := q.ExecContext(ctx, `INSERT INTO label_items (path, dir, colour, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT (path) DO UPDATE SET colour = excluded.colour, updated_at = excluded.updated_at`,
		p, path.Dir(p), colour, time.Now().Unix()); err != nil {
		return err
	}
	for _, t := range l.Tags {
		if _, err := q.ExecContext(ctx, `INSERT OR IGNORE INTO label_item_tags (path, tag) VALUES (?, ?)`, p, t); err != nil {
			return err
		}
	}
	return nil
}

// Move names a path before and after a rename, move or copy.
type Move struct {
	From, To string
}

// transfer gives every To the labels of its From, including everything
// below it. What To carried before is replaced, as the file there was. It
// returns the folders whose entries changed.
func transfer(ctx context.Context, tx *sql.Tx, moves []Move, keep bool) ([]string, error) {
	var touched []string
	for _, m := range moves {
		src, err := loadTree(ctx, tx, m.From)
		if err != nil {
			return nil, err
		}
		gone, err := dropTree(ctx, tx, m.To)
		if err != nil {
			return nil, err
		}
		if !keep {
			if _, err := dropTree(ctx, tx, m.From); err != nil {
				return nil, err
			}
		}
		for p, l := range src {
			if err := put(ctx, tx, m.To+strings.TrimPrefix(p, m.From), l); err != nil {
				return nil, err
			}
		}
		if len(src) > 0 || gone > 0 {
			touched = append(touched, path.Dir(m.To))
		}
		if len(src) > 0 && !keep {
			touched = append(touched, path.Dir(m.From))
		}
	}
	return touched, nil
}

func counts(ctx context.Context, db *sql.DB) (colours, tags map[string]int, err error) {
	colours, tags = map[string]int{}, map[string]int{}
	scan := func(query string, into map[string]int) error {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k string
			var n int
			if err := rows.Scan(&k, &n); err != nil {
				return err
			}
			into[k] = n
		}
		return rows.Err()
	}
	if err := scan(`SELECT colour, COUNT(*) FROM label_items WHERE colour IS NOT NULL GROUP BY colour`, colours); err != nil {
		return nil, nil, err
	}
	if err := scan(`SELECT tag, COUNT(*) FROM label_item_tags GROUP BY tag`, tags); err != nil {
		return nil, nil, err
	}
	return colours, tags, nil
}
