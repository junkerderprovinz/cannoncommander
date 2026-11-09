package main

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/godbus/dbus/v5"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/flatpak"
)

const (
	folderMime = "inode/directory"
	// entryName is the desktop entry that opens folders here. It stays out
	// of the menu, where the installer's own entry stands.
	entryName = "cannoncommander-folders.desktop"
	// previousKey keeps the entry that opened folders before, to give the
	// place back to.
	previousKey = "X-CannonCommander-Previous="
	// fileManager1 is the service other programs ask to show a file in its
	// folder, as a browser does for a download.
	fileManager1 = "org.freedesktop.FileManager1"
)

// xdgHandler follows the freedesktop specifications in the person's own
// folders: the default for inode/directory, which every desktop asks, and a
// D-Bus service file, so a request to show a folder starts the app when it
// is not running.
type xdgHandler struct {
	apps     string
	services string
	mimeapps string
	// query reads the default for folders and assign sets it, which
	// xdg-mime does for the desktop in use.
	query  func() string
	assign func(entry string) error
	// reload has the session bus read its service files again.
	reload func()
}

// newFolderHandler is nil in a Flatpak, which can neither write the entry
// and the service the computer reads nor own the FileManager1 name. Its own
// desktop entry names it for folders, so the system settings can make it
// the default there.
func newFolderHandler() folderHandler {
	if flatpak.ID() != "" {
		return nil
	}
	dirs := currentDirs()
	if len(dirs.config) == 0 {
		return nil
	}
	return xdgHandler{
		apps:     filepath.Join(dirs.data[0], "applications"),
		services: filepath.Join(dirs.data[0], "dbus-1", "services"),
		mimeapps: filepath.Join(dirs.config[0], "mimeapps.list"),
		// xdg-mime 1.1.3, as Ubuntu 24.04 has it, misreads a quoted Exec
		// line and answers with another entry, so the lists are read here.
		query:  func() string { return defaultFor(folderMime, currentDirs()) },
		assign: func(entry string) error { return exec.Command("xdg-mime", "default", entry, folderMime).Run() },
		reload: reloadBus,
	}
}

func (h xdgHandler) set(exe string, w desktop.Words) error {
	previous, err := h.previous()
	if err != nil {
		previous = h.query()
	}
	if previous == entryName {
		previous = ""
	}
	comment := w.Open
	if comment == "" {
		comment = "Open folders in CannonCommander"
	}
	entry := "[Desktop Entry]\nType=Application\nName=CannonCommander\nComment=" + oneLine(comment) +
		"\nExec=" + quoteExec(exe) + " %U\nMimeType=" + folderMime + ";\nNoDisplay=true\n" + previousKey + previous + "\n"
	if err := writeFile(filepath.Join(h.apps, entryName), entry); err != nil {
		return err
	}
	service := "[D-BUS Service]\nName=" + fileManager1 + "\nExec=" + quoteService(exe) + "\n"
	if err := writeFile(filepath.Join(h.services, fileManager1+".service"), service); err != nil {
		return err
	}
	h.reload()
	return h.assign(entryName)
}

func (h xdgHandler) unset() error {
	previous, err := h.previous()
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if previous != "" {
		err = h.assign(previous)
	} else {
		err = h.forget()
	}
	err = errors.Join(err,
		ignoreMissing(os.Remove(filepath.Join(h.apps, entryName))),
		ignoreMissing(os.Remove(filepath.Join(h.services, fileManager1+".service"))))
	h.reload()
	return err
}

func (h xdgHandler) isSet(exe string) bool {
	body, err := os.ReadFile(filepath.Join(h.apps, entryName))
	return err == nil && h.query() == entryName && bytes.Contains(body, []byte("Exec="+quoteExec(exe)+" "))
}

// previous reads which entry opened folders before this app took over.
func (h xdgHandler) previous() (string, error) {
	f, err := os.Open(filepath.Join(h.apps, entryName))
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), previousKey); ok {
			return v, nil
		}
	}
	return "", sc.Err()
}

// forget takes the app's entry out of mimeapps.list when no other entry had
// folders before, so the desktop falls back to its own file manager.
func (h xdgHandler) forget() error {
	body, err := os.ReadFile(h.mimeapps)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		if v, ok := strings.CutPrefix(line, folderMime+"="); ok {
			var keep []string
			for _, e := range strings.Split(v, ";") {
				if e != "" && e != entryName {
					keep = append(keep, e)
				}
			}
			if len(keep) == 0 {
				continue
			}
			line = folderMime + "=" + strings.Join(keep, ";") + ";"
		}
		out = append(out, line)
	}
	return os.WriteFile(h.mimeapps, []byte(strings.Join(out, "\n")), 0o644)
}

// xdgDirs are the folders of the XDG base directory specification, the
// person's own first, and the desktops named in XDG_CURRENT_DESKTOP in lower
// case.
type xdgDirs struct {
	config, data []string
	desktops     []string
}

func currentDirs() xdgDirs {
	home, err := os.UserHomeDir()
	if err != nil {
		return xdgDirs{}
	}
	list := func(own, ownDefault, others, othersDefault string) []string {
		dirs := []string{cmp.Or(os.Getenv(own), filepath.Join(home, ownDefault))}
		for _, d := range filepath.SplitList(cmp.Or(os.Getenv(others), othersDefault)) {
			if filepath.IsAbs(d) {
				dirs = append(dirs, d)
			}
		}
		return dirs
	}
	var desktops []string
	for _, d := range strings.Split(os.Getenv("XDG_CURRENT_DESKTOP"), ":") {
		if d != "" {
			desktops = append(desktops, strings.ToLower(d))
		}
	}
	return xdgDirs{
		config:   list("XDG_CONFIG_HOME", ".config", "XDG_CONFIG_DIRS", "/etc/xdg"),
		data:     list("XDG_DATA_HOME", ".local/share", "XDG_DATA_DIRS", "/usr/local/share:/usr/share"),
		desktops: desktops,
	}
}

// defaultFor reads which desktop entry opens mime the way GLib does: the
// mimeapps.list files in the order of the MIME applications specification,
// and in them the first default that is installed.
func defaultFor(mime string, d xdgDirs) string {
	var lists []string
	add := func(dir string) {
		for _, desk := range d.desktops {
			lists = append(lists, filepath.Join(dir, desk+"-mimeapps.list"))
		}
		lists = append(lists, filepath.Join(dir, "mimeapps.list"))
	}
	for _, dir := range d.config {
		add(dir)
	}
	for _, dir := range d.data {
		add(filepath.Join(dir, "applications"))
	}
	for _, list := range lists {
		for _, entry := range defaults(list, mime) {
			if installed(entry, d.data) {
				return entry
			}
		}
	}
	return ""
}

// defaults are the entries one mimeapps.list makes the default for mime.
func defaults(list, mime string) []string {
	f, err := os.Open(list)
	if err != nil {
		return nil
	}
	defer f.Close()
	var group string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if strings.HasPrefix(line, "[") {
			group = line
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && group == "[Default Applications]" && strings.TrimSpace(key) == mime {
			var entries []string
			for _, e := range strings.Split(value, ";") {
				if e = strings.TrimSpace(e); e != "" {
					entries = append(entries, e)
				}
			}
			return entries
		}
	}
	return nil
}

// installed reports whether a desktop entry id names a file in one of the
// applications folders. A dash in the id may stand for a subfolder.
func installed(id string, data []string) bool {
	for _, dir := range data {
		for _, name := range []string{id, strings.Replace(id, "-", "/", 1)} {
			if _, err := os.Stat(filepath.Join(dir, "applications", name)); err == nil {
				return true
			}
		}
	}
	return false
}

// reloadBus has the session bus read its service files again. A bus that
// started before the services folder existed does not watch it, and would
// otherwise not know who to start for FileManager1.
func reloadBus() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return
	}
	defer conn.Close()
	if err := conn.BusObject().Call("org.freedesktop.DBus.ReloadConfig", 0).Err; err != nil {
		log.Printf("the session bus did not reload its services: %v", err)
	}
}

// quoteExec writes a program for the Exec key of a desktop entry. The
// specification quotes an argument only for a reserved character, and
// xdg-utils before 1.2 cannot read a quoted program at all. The escapes of a
// string value come on top of the quoting.
func quoteExec(p string) string {
	if strings.ContainsAny(p, " \t\n\"'\\><~|&;$*?#()`") {
		p = `"` + strings.NewReplacer(`"`, `\"`, "`", "\\`", "$", `\$`, `\`, `\\`).Replace(p) + `"`
	}
	return strings.NewReplacer(`\`, `\\`, "%", "%%").Replace(p)
}

// quoteService writes a program for the Exec key of a D-Bus service file,
// which the bus splits as a shell does.
func quoteService(p string) string {
	if !strings.ContainsAny(p, " \t\n\"'\\><~|&;$*?#()`") {
		return p
	}
	return "'" + strings.ReplaceAll(p, "'", `'\''`) + "'"
}

func oneLine(s string) string { return strings.NewReplacer("\n", " ", "\r", " ").Replace(s) }

func writeFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

func ignoreMissing(err error) error {
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}
