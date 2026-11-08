package launch

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

var errNoOpener = errors.New("neither gio nor xdg-open is installed")

type desktopSystem struct{}

func newSystem() system { return desktopSystem{} }

func (desktopSystem) asks(file string) bool { return startsSomething(file) }

func (desktopSystem) open(file string) error {
	for _, c := range [][]string{{"gio", "open"}, {"xdg-open"}} {
		if p, err := exec.LookPath(c[0]); err == nil {
			return start(exec.Command(p, append(c[1:], file)...))
		}
	}
	return errNoOpener
}

func (desktopSystem) run(file string) error {
	if strings.HasSuffix(file, ".desktop") {
		p, err := exec.LookPath("gio")
		if err != nil {
			return err
		}
		return start(exec.Command(p, "launch", file))
	}
	c := exec.Command(file)
	c.Dir = filepath.Dir(file)
	return start(c)
}

// startsSomething reports whether a double click on file starts it rather
// than opening it: a desktop entry, or a binary or script that may run.
func startsSomething(file string) bool {
	if strings.HasSuffix(file, ".desktop") {
		return true
	}
	info, err := os.Stat(file)
	if err != nil || info.Mode()&0o111 == 0 {
		return false
	}
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()
	head := make([]byte, 4)
	n, _ := io.ReadFull(f, head)
	head = head[:n]
	return bytes.HasPrefix(head, []byte("\x7fELF")) || bytes.HasPrefix(head, []byte("#!"))
}

// start runs c in a session of its own, so it outlives the app, and reports
// a failure that shows within a few seconds. gio and xdg-open hand the file
// on and end, while a program that started keeps running.
func start(c *exec.Cmd) error {
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := c.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		return err
	case <-time.After(3 * time.Second):
		return nil
	}
}
