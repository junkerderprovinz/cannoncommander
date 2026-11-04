//go:build !windows

package system

import (
	"os"
	"syscall"

	"github.com/gtsteffaniak/go-logger/logger"
)

func stopSelf() {
	if err := syscall.Kill(os.Getpid(), syscall.SIGTERM); err != nil {
		logger.Errorf("restart: %v", err)
	}
}

func reexec(exe string, args, env []string) error {
	return syscall.Exec(exe, args, env)
}
