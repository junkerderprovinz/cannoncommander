package nativeshell

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// dolphin lists the actions of Dolphin's service menus that fit sel. Where
// two folders hold a menu of the same name, the one that comes first wins.
func (k *desk) dolphin(sel selection) []action {
	var out []action
	seen := map[string]bool{}
	for i, data := range k.Data {
		for _, sub := range []string{"kio/servicemenus", "kservices5/ServiceMenus"} {
			dir := filepath.Join(data, sub)
			files, _ := os.ReadDir(dir)
			for _, f := range files {
				name := f.Name()
				if seen[name] || !strings.HasSuffix(name, ".desktop") {
					continue
				}
				seen[name] = true
				file := filepath.Join(dir, name)
				// KIO runs a menu from the user's own folder only when it is
				// executable, so a file that merely landed there does nothing.
				if i == 0 && !executable(file) {
					continue
				}
				kf, err := xdg.ReadKeyFile(file)
				if err != nil || !k.serviceMenuFits(kf["Desktop Entry"], sel) {
					continue
				}
				group := k.Localized(kf["Desktop Entry"], "X-KDE-Submenu")
				for _, id := range kf["Desktop Entry"].List("Actions") {
					g := kf["Desktop Action "+id]
					e := xdg.Entry{File: file, Name: k.Localized(g, "Name"), Icon: g.String("Icon"), Exec: g.String("Exec")}
					if e.Name == "" || e.Exec == "" {
						continue
					}
					out = append(out, action{
						Action: Action{Entry: Entry{Key: "dolphin:" + name + "#" + id, Label: e.Name}, Group: group},
						icon:   e.Icon,
						launches: func(sel selection) ([]launch, error) {
							runs, err := xdg.Expand(e, sel.paths())
							if err != nil {
								return nil, err
							}
							var out []launch
							for _, args := range runs {
								out = append(out, launch{args: args, dir: sel.dir})
							}
							return out, nil
						},
					})
				}
			}
		}
	}
	return out
}

// serviceMenuFits reports whether a service menu is for sel: files on disk,
// as many as it takes, each of a type it names. A menu that shows only
// while some program runs is left out, as that takes asking D-Bus.
func (k *desk) serviceMenuFits(g xdg.Group, sel selection) bool {
	if g["Type"] != "Service" || g["X-KDE-ShowIfRunning"] != "" || g["X-KDE-ShowIfDBusCall"] != "" {
		return false
	}
	if protocols := kdeList(g, "X-KDE-Protocols", "X-KDE-Protocol"); len(protocols) > 0 && !slices.Contains(protocols, "file") {
		return false
	}
	n := len(sel.items)
	if least, err := strconv.Atoi(g["X-KDE-MinNumberOfUrls"]); err == nil && n < least {
		return false
	}
	if most, err := strconv.Atoi(g["X-KDE-MaxNumberOfUrls"]); err == nil && n > most {
		return false
	}
	if counts := kdeList(g, "X-KDE-RequiredNumberOfUrls"); len(counts) > 0 && !slices.Contains(counts, strconv.Itoa(n)) {
		return false
	}
	var types []string
	for _, t := range kdeList(g, "MimeType", "ServiceTypes", "X-KDE-ServiceTypes") {
		if strings.Contains(t, "/") && !strings.HasPrefix(t, "KonqPopupMenu/") {
			types = append(types, t)
		}
	}
	excluded := kdeList(g, "ExcludeServiceTypes")
	for _, it := range sel.items {
		of := func(t string) bool { return k.serviceType(it, t) }
		if !slices.ContainsFunc(types, of) || slices.ContainsFunc(excluded, of) {
			return false
		}
	}
	return len(types) > 0
}

// serviceType reports whether it is of type t as KIO sees it, where
// all/all is anything, and all/allfiles and application/octet-stream are
// any file but no folder.
func (k *desk) serviceType(it item, t string) bool {
	switch t {
	case "all/all":
		return true
	case "all/allfiles", "application/octet-stream":
		return !it.dir
	}
	return k.is(it, t)
}

// kdeList reads lists that KDE writes with semicolons or with commas.
func kdeList(g xdg.Group, keys ...string) []string {
	var out []string
	for _, key := range keys {
		for _, item := range g.List(key) {
			for _, part := range strings.Split(item, ",") {
				if part = strings.TrimSpace(part); part != "" {
					out = append(out, part)
				}
			}
		}
	}
	return out
}
