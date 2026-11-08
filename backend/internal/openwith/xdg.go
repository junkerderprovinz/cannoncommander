package openwith

import (
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// apps lists the programs for file the way GNOME and KDE order them: the
// default, the ones a person added, then every program that names the type,
// and after them the ones for the types it is a kind of.
func apps(d xdg.Desktop, file string) []app {
	all := d.Entries()
	db := d.MimeDB()
	var def string
	var ids []string
	for _, m := range db.Lineage(db.TypeOf(file)) {
		removed := map[string]bool{}
		for _, a := range d.MimeApps(db, m) {
			for _, id := range a.Removed {
				removed[id] = true
			}
			for _, id := range a.Default {
				if _, ok := all[id]; ok && def == "" && !removed[id] {
					def = id
				}
			}
			for _, id := range a.Added {
				if !removed[id] {
					ids = append(ids, id)
				}
			}
		}
		var named []xdg.Entry
		for _, e := range all {
			if !removed[e.ID] && slices.ContainsFunc(e.Mime, func(t string) bool { return db.Canonical(t) == m }) {
				named = append(named, e)
			}
		}
		slices.SortFunc(named, byName)
		for _, e := range named {
			ids = append(ids, e.ID)
		}
	}
	if def != "" {
		ids = append([]string{def}, ids...)
	}
	var out []app
	have := map[string]bool{}
	for _, id := range ids {
		e, ok := all[id]
		if !ok || have[id] {
			continue
		}
		have[id] = true
		out = append(out, app{id: id, name: e.Name, icon: e.Icon, def: id == def})
	}
	return out
}

// every lists the programs a desktop shows in its menus, by name.
func every(d xdg.Desktop) []app {
	var shown []xdg.Entry
	for _, e := range d.Entries() {
		if !e.NoDisplay {
			shown = append(shown, e)
		}
	}
	slices.SortFunc(shown, byName)
	out := make([]app, 0, len(shown))
	for _, e := range shown {
		out = append(out, app{id: e.ID, name: e.Name, icon: e.Icon})
	}
	return out
}

func byName(a, b xdg.Entry) int {
	if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}
