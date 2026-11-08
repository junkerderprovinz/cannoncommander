//go:build desktop && !windows

package terminal

import (
	"os"
	"os/exec"
	"syscall"
)

// startNative runs l in a session of its own, so the terminal stays open when
// the app quits, and leaves it running. The wait in the background collects
// the process once its window closes.
func startNative(l launch) error {
	cmd := exec.Command(l.program, l.args...)
	cmd.Dir = l.dir
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
