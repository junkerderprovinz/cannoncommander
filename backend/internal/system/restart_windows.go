package system

import (
	"os"
	"os/exec"
)

// Windows has no signal to shut down gracefully and no exec, so a
// restart there runs the hooks, starts a new process and ends this one.
func stopSelf() {
	Shutdown()
	exe, err := os.Executable()
	if err == nil {
		err = reexec(exe, os.Args, os.Environ())
	}
	if err == nil {
		os.Exit(0)
	}
}

func reexec(exe string, args, env []string) error {
	cmd := exec.Command(exe, args[1:]...)
	cmd.Env, cmd.Stdout, cmd.Stderr = env, os.Stdout, os.Stderr
	return cmd.Start()
}
