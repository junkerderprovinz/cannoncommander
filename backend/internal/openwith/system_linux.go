//go:build desktop

package openwith

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/xdg"
)

type desktopEntries struct{}

func newSystem() system { return desktopEntries{} }

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
func (desktopEntries) open(file, id string) error {
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
