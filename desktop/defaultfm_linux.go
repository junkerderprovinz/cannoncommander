package main

import (
	"bufio"
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
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
	// query and assign read and set the default for folders, which
	// xdg-mime does for the desktop in use.
	query  func() string
	assign func(entry string) error
}

func newFolderHandler() folderHandler {
	data := os.Getenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	if data == "" {
		data = filepath.Join(home, ".local", "share")
	}
	config, err := os.UserConfigDir()
	if err != nil {
		return nil
	}
	return xdgHandler{
		apps:     filepath.Join(data, "applications"),
		services: filepath.Join(data, "dbus-1", "services"),
		mimeapps: filepath.Join(config, "mimeapps.list"),
		query: func() string {
			out, _ := exec.Command("xdg-mime", "query", "default", folderMime).Output()
			return strings.TrimSpace(string(out))
		},
		assign: func(entry string) error { return exec.Command("xdg-mime", "default", entry, folderMime).Run() },
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
	service := "[D-BUS Service]\nName=" + fileManager1 + "\nExec=" + quoteExec(exe) + "\n"
	if err := writeFile(filepath.Join(h.services, fileManager1+".service"), service); err != nil {
		return err
	}
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
	return errors.Join(err,
		ignoreMissing(os.Remove(filepath.Join(h.apps, entryName))),
		ignoreMissing(os.Remove(filepath.Join(h.services, fileManager1+".service"))))
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

// quoteExec quotes a program for the Exec key, as the desktop entry
// specification asks for a path with spaces or other reserved characters.
func quoteExec(p string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "`", "\\`", "$", `\$`)
	return `"` + r.Replace(p) + `"`
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
