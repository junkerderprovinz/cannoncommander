package nativeshell

import (
	"cmp"
	"encoding/base64"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

// actionIconSize is the edge of an action's icon in pixels, twice the glyph
// of a menu row.
const actionIconSize = 32

// action is a row of another file manager's menu and what it runs.
type action struct {
	Action
	// icon is the name of the row's icon in the theme, or its file.
	icon     string
	launches func(selection) ([]launch, error)
}

// launch is one program an action starts.
type launch struct {
	args []string
	dir  string
	env  []string
}

// NativeMenu reports whether the system draws the menu of files itself,
// which no Linux desktop offers to another program.
func NativeMenu() bool { return false }

// ShowMenu returns ErrUnsupported, as Linux has no menu of files to show.
func ShowMenu([]string, MenuOptions) ([]Entry, error) { return nil, ErrUnsupported }

// Actions lists the rows that Dolphin's service menus, Nemo's actions,
// Nautilus's scripts and Thunar's custom actions offer for files, sorted
// by submenu and name.
func Actions(files []string) ([]Action, error) {
	k := current()
	sel, err := k.selection(files)
	if err != nil {
		return nil, err
	}
	found := k.actions(sel)
	out := make([]Action, 0, len(found))
	for _, a := range found {
		a.Icon = k.iconURL(a.icon)
		out = append(out, a.Action)
	}
	return out, nil
}

// RunAction starts the action with key on files, provided it is one of
// theirs. No shell runs it, so every file name stays one argument.
func RunAction(files []string, key string) error {
	k := current()
	sel, err := k.selection(files)
	if err != nil {
		return err
	}
	all := k.actions(sel)
	i := slices.IndexFunc(all, func(a action) bool { return a.Key == key })
	if i < 0 {
		return ErrUnknownAction
	}
	launches, err := all[i].launches(sel)
	if err != nil {
		return err
	}
	for _, l := range launches {
		if err := start(l); err != nil {
			return err
		}
	}
	return nil
}

func (k *desk) actions(sel selection) []action {
	all := slices.Concat(k.dolphin(sel), k.nemo(sel), k.nautilus(sel), k.thunar(sel))
	slices.SortFunc(all, func(a, b action) int {
		return cmp.Or(
			strings.Compare(strings.ToLower(a.Group), strings.ToLower(b.Group)),
			strings.Compare(strings.ToLower(a.Label), strings.ToLower(b.Label)),
			strings.Compare(a.Key, b.Key),
		)
	})
	return all
}

func start(l launch) error {
	cmd := exec.Command(l.args[0], l.args[1:]...)
	cmd.Dir = l.dir
	cmd.Env = append(append(os.Environ(), "PWD="+l.dir), l.env...)
	// A session of its own keeps the program open when the app quits.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

var actionIcons = struct {
	sync.Mutex
	urls map[string]string
}{urls: map[string]string{}}

// iconURL is the icon an action names as a data URL, or "". It is kept,
// since every menu asks for the same few again.
func (k *desk) iconURL(name string) string {
	if name == "" {
		return ""
	}
	key := k.theme + "/" + name
	actionIcons.Lock()
	defer actionIcons.Unlock()
	if u, ok := actionIcons.urls[key]; ok {
		return u
	}
	u := ""
	if file, ok := k.namedIcon(name, actionIconSize); ok {
		if b, mediaType, err := xdg.LoadIcon(file, actionIconSize); err == nil {
			u = "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(b)
		}
	}
	actionIcons.urls[key] = u
	return u
}
