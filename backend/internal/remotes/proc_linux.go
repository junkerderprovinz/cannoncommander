package remotes

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// childAttr ties rcd to the server, so a crash of the server also ends rcd
// and with it the mounts, instead of leaving them behind without an owner.
func childAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM}
}

// stopChild asks rcd to end, which unmounts everything first.
func stopChild(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}

// clearStale unmounts a mount whose rclone is gone. Its folder then answers
// every call with ENOTCONN, and nothing can be mounted on it again.
func clearStale(dir string) {
	if _, err := os.Stat(dir); !errors.Is(err, syscall.ENOTCONN) {
		return
	}
	for _, tool := range [][]string{{"fusermount3", "-uz"}, {"fusermount", "-uz"}, {"umount", "-l"}} {
		if exec.Command(tool[0], append(tool[1:], dir)...).Run() == nil {
			return
		}
	}
}
