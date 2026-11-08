package system

import (
	"os"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

var shutdown struct {
	mu    sync.Mutex
	hooks []func()
}

// OnShutdown adds fn to what Shutdown runs, such as ending a child process
// that holds mounts.
func OnShutdown(fn func()) {
	shutdown.mu.Lock()
	shutdown.hooks = append(shutdown.hooks, fn)
	shutdown.mu.Unlock()
}

// Shutdown runs the hooks, the last added first. The server calls it once
// the web server stopped.
func Shutdown() {
	shutdown.mu.Lock()
	hooks := slices.Clone(shutdown.hooks)
	shutdown.hooks = nil
	shutdown.mu.Unlock()
	for _, fn := range slices.Backward(hooks) {
		fn()
	}
}

var restartWanted atomic.Bool

// RestartWanted reports whether an admin asked for a restart. The server
// then starts itself again after its normal shutdown.
func RestartWanted() bool { return restartWanted.Load() }

// Restart replaces the process with a fresh start of the same program, so
// the container keeps running.
func Restart() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return reexec(exe, os.Args, os.Environ())
}

// requestRestart lets the answer reach the browser first and then shuts
// the server down the way a stop signal would. The desktop app ends its
// window with it and starts itself again.
func requestRestart() {
	if app := desktop.Current(); app != nil {
		time.AfterFunc(500*time.Millisecond, app.Restart)
		return
	}
	restartWanted.Store(true)
	time.AfterFunc(500*time.Millisecond, stopSelf)
}
