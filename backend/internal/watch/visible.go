package watch

import (
	"encoding/json"
	"path"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// ForUser drops the entries of an fs.changed event that u may not see. The
// event goes to every client watching the folder, and access rules can hide
// single entries from some of them.
func ForUser(fs *ccfs.Resolver, u *users.User, e hub.Event) hub.Event {
	if e.Type != EventChanged {
		return e
	}
	var c Change
	if json.Unmarshal(e.Data, &c) != nil {
		return e
	}
	dir, err := fs.Resolve(c.Path)
	if err != nil {
		return e
	}
	see := func(name string) bool { return fs.Visible(u, dir, name) }
	hidden := false
	keep := func(names []string) []string {
		out := names[:0]
		for _, n := range names {
			if see(n) {
				out = append(out, n)
			} else {
				hidden = true
			}
		}
		return out
	}
	c.Names = keep(c.Names)
	c.Removed = keep(c.Removed)
	paths := c.Paths[:0]
	for i, p := range c.Paths {
		if i == 0 || see(path.Base(p)) {
			paths = append(paths, p)
		} else {
			hidden = true
		}
	}
	c.Paths = paths
	renamed := c.Renamed[:0]
	for _, r := range c.Renamed {
		from, to := see(r.From), see(r.To)
		switch {
		case from && to:
			renamed = append(renamed, r)
			continue
		case to:
			c.Names = append(c.Names, r.To)
		case from:
			c.Removed = append(c.Removed, r.From)
		}
		hidden = true
	}
	c.Renamed = renamed
	if !hidden {
		return e
	}
	if data, err := json.Marshal(c); err == nil {
		e.Data = data
	}
	return e
}
