package openwith

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type desktopEntries struct{}

func newSystem() system { return desktopEntries{} }

func current() xdg { return xdgFrom(os.Getenv, exec.LookPath) }

// Linux has no dialog of its own, so the app lists every program instead.
func (desktopEntries) dialog() bool { return false }

func (desktopEntries) choose(string) error { return errNoDialog }

func (desktopEntries) apps(file string) ([]app, error) { return current().apps(file), nil }

func (desktopEntries) every() ([]app, error) { return current().every(), nil }

func (desktopEntries) icon(where string) (string, error) {
	file, err := current().iconFile(where)
	if err != nil {
		return "", err
	}
	if strings.HasSuffix(file, ".svg") {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		return dataURL("image/svg+xml", b), nil
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return "", err
	}
	return pngIcon(img)
}

// open starts the program through gio where GLib has it, which also knows
// programs that run in a terminal or start over D-Bus, and by its Exec line
// elsewhere.
func (desktopEntries) open(file, id string) error {
	e, ok := current().entries()[id]
	if !ok {
		return errUnknownApp
	}
	if gio, err := exec.LookPath("gio"); err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, gio, "launch", e.file, file).CombinedOutput()
		if err != nil {
			return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
		}
		return nil
	}
	args, err := command(e, file)
	if err != nil {
		return err
	}
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
