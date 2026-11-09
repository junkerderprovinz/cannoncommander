//go:build desktop

package openwith

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/flatpak"
)

type desktopEntries struct{}

func newSystem() system {
	if flatpak.ID() != "" {
		return portalApps{ask: askPortal}
	}
	return desktopEntries{}
}

// portalApps is a Flatpak's way to the computer's programs, which are
// outside its sandbox: the desktop's portal shows its own dialog to pick
// one, and the app lists none itself.
type portalApps struct {
	desktopEntries
	ask func(file string) error
}

func (portalApps) dialog() bool                      { return true }
func (p portalApps) choose(file string) error        { return p.ask(file) }
func (portalApps) apps(string) ([]app, error)        { return nil, nil }
func (portalApps) every() ([]app, error)             { return nil, nil }
func (portalApps) open(string, string, string) error { return errUnknownApp }

// askPortal hands file to the desktop portal with the request to ask which
// program opens it. The portal takes the open file rather than its path,
// which a program outside the sandbox might not reach.
func askPortal(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	options := map[string]dbus.Variant{"ask": dbus.MakeVariant(true)}
	return conn.Object("org.freedesktop.portal.Desktop", "/org/freedesktop/portal/desktop").
		CallWithContext(ctx, "org.freedesktop.portal.OpenURI.OpenFile", 0, "", dbus.UnixFD(f.Fd()), options).Err
}

// Linux has no dialog of its own, so the app lists every program instead.
func (desktopEntries) dialog() bool { return false }

func (desktopEntries) choose(string) error { return errNoDialog }

func (desktopEntries) apps(file string) ([]app, error) { return apps(xdg.Current(), file), nil }

func (desktopEntries) every() ([]app, error) { return every(xdg.Current()), nil }

func (desktopEntries) icon(where string) (string, error) {
	file, ok := xdg.Current().AppIcon(where)
	if !ok {
		return "", errNoIcon
	}
	b, mediaType, err := xdg.LoadIcon(file, iconSize)
	if err != nil {
		return "", err
	}
	return dataURL(mediaType, b), nil
}

// open starts the program through gio where GLib has it, which also knows
// programs that run in a terminal or start over D-Bus, and by its Exec line
// elsewhere.
func (desktopEntries) open(file, id, _ string) error {
	e, ok := xdg.Current().Entries()[id]
	if !ok {
		return errUnknownApp
	}
	if gio, err := exec.LookPath("gio"); err == nil {
		return gioLaunch(gio, e.File, file)
	}
	runs, err := xdg.Expand(e, []string{file})
	if err != nil {
		return err
	}
	args := runs[0]
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = filepath.Dir(file)
	// A session of its own keeps the program open when the app quits.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// pick has nothing to show: Linux has no dialog for programs, so the app
// lists every program instead.
func (desktopEntries) pick(_, _ string) (app, error) { return app{}, errNoDialog }

func (desktopEntries) runs(_ string, info os.FileInfo) bool {
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func (desktopEntries) run(program, file string) error {
	cmd := exec.Command(program, file)
	cmd.Dir = filepath.Dir(file)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// gioLaunch starts the program of a desktop entry with gio. The program
// inherits gio's output, so a pipe there would hold the request until the
// program quits, and a pipe closed early could kill the program on its next
// write. gio's output goes nowhere.
func gioLaunch(gio, entry, file string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, gio, "launch", entry, file).Run(); err != nil {
		return fmt.Errorf("gio launch %s: %w", filepath.Base(entry), err)
	}
	return nil
}
