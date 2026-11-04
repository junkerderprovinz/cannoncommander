package search

import (
	"context"
	"database/sql"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// colourNames are what the six colours that come with the app are called
// as long as nobody renames them, the way #rot or #grün find them.
var colourNames = map[string]string{"red": "rot", "orange": "orange", "yellow": "gelb", "green": "grün", "blue": "blau", "purple": "lila"}

// taggedPaths reads which paths carry one of the tags or colours, so a
// #word costs one query per search and none per entry.
func taggedPaths(ctx context.Context, db *sql.DB, words []string) func(path, tag string) bool {
	none := func(string, string) bool { return false }
	if db == nil {
		return none
	}
	want := map[string]bool{}
	for _, w := range words {
		want[strings.ToLower(w)] = true
	}
	hits := map[string]map[string]bool{}
	mark := func(p, word string) {
		if hits[p] == nil {
			hits[p] = map[string]bool{}
		}
		hits[p][word] = true
	}
	rows, err := db.QueryContext(ctx, `SELECT path, tag FROM label_item_tags`)
	if err != nil {
		logger.Debugf("search tags: %v", err)
		return none
	}
	for rows.Next() {
		var p, tag string
		if rows.Scan(&p, &tag) == nil && want[strings.ToLower(tag)] {
			mark(p, strings.ToLower(tag))
		}
	}
	rows.Close()
	colours := map[string]string{}
	if rows, err := db.QueryContext(ctx, `SELECT id, name FROM label_colours`); err == nil {
		for rows.Next() {
			var id, name string
			if rows.Scan(&id, &name) != nil {
				continue
			}
			if name == "" {
				name = colourNames[id]
			}
			if want[strings.ToLower(name)] {
				colours[id] = strings.ToLower(name)
			}
		}
		rows.Close()
	}
	if len(colours) > 0 {
		if rows, err := db.QueryContext(ctx, `SELECT path, colour FROM label_items WHERE colour IS NOT NULL`); err == nil {
			for rows.Next() {
				var p, c string
				if rows.Scan(&p, &c) == nil && colours[c] != "" {
					mark(p, colours[c])
				}
			}
			rows.Close()
		}
	}
	return func(p, tag string) bool { return hits[p][tag] }
}
