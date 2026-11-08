package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/cmd"
	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

// The first check waits for the window and the server to settle; one a day
// follows. An installed copy looks every followEvery for a version the
// scheduled task has put in place.
var (
	updateAPI   = "https://api.github.com"
	firstCheck  = time.Minute
	followEvery = 10 * time.Minute
)

const checkEvery = 24 * time.Hour

// updater keeps the desktop app current. It is the only caller of
// update.Updater in its process, so there is never more than one update in
// flight.
type updater struct {
	u     *update.Updater
	log   *log.Logger
	// ready is the version waiting for the next start, which the Info page
	// offers a restart for.
	ready atomic.Pointer[string]
	// swapping is held while the program is being replaced, and by stop for
	// good, so the process never exits halfway through a swap.
	swapping sync.Mutex
}

func newUpdater(logger *log.Logger) *updater {
	return &updater{
		log: logger,
		u: &update.Updater{
			Repo:    update.Repo,
			Version: cmd.Version(),
			Assets:  update.Assets,
			API:     updateAPI,
			// Long enough for the zip on a slow line, short enough that a
			// connection that stalls does not hold up every later check.
			Client: &http.Client{Timeout: 30 * time.Minute},
			Logf:   logger.Printf,
		},
	}
}

// openUpdateLog appends to the log at path, since a windowed program on
// Windows has no console to say why an update did not happen. A log grown past
// 256 KiB starts over.
func openUpdateLog(path string) *log.Logger {
	var w io.Writer = os.Stderr
	if os.MkdirAll(filepath.Dir(path), 0o755) == nil {
		flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND
		if info, err := os.Stat(path); err == nil && info.Size() > 256<<10 {
			flags |= os.O_TRUNC
		}
		if f, err := os.OpenFile(path, flags, 0o644); err == nil {
			w = f
		}
	}
	return log.New(w, "", log.LstdFlags)
}

// run removes what the last update left and then checks for updates until ctx
// ends, whenever autoUpdate allows. A build that does not know which release
// it is never checks.
func (up *updater) run(ctx context.Context, autoUpdate func() bool) {
	up.u.Cleanup()
	if !update.IsRelease(cmd.Version()) {
		up.log.Printf("update: %s is not a release build, so it does not update itself", cmd.Version())
		return
	}
	timer := time.NewTimer(firstCheck)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if autoUpdate() {
			up.once(ctx)
		}
		timer.Reset(checkEvery)
	}
}

// follow is run for the installed copy, which cannot replace itself. It waits
// for the scheduled task to record a newer version in the uninstall entry and
// says once that it starts next time.
func (up *updater) follow(ctx context.Context) {
	tick := time.NewTicker(followEvery)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}
		if v := update.InstalledVersion(product); update.Newer(v, cmd.Version()) {
			up.ready.Store(&v)
			return
		}
	}
}

func (up *updater) once(ctx context.Context) {
	rel, err := up.u.Check(ctx)
	if err != nil {
		up.log.Printf("update: %v", err)
		return
	}
	if rel == nil {
		up.log.Printf("update: no newer release")
		return
	}
	download, err := up.u.Fetch(ctx, rel)
	if err != nil {
		up.log.Printf("update: not updating to %s: %v", rel.Version, err)
		return
	}
	defer os.Remove(download)

	up.swapping.Lock()
	err = up.u.Swap(download, rel)
	up.swapping.Unlock()
	if err != nil {
		up.log.Printf("update: not updating to %s: %v", rel.Version, err)
		return
	}
	up.log.Printf("update: %s is in place and starts next time", rel.Version)
	up.ready.Store(&rel.Version)
}

// readyVersion is the version waiting for the next start, or "".
func (up *updater) readyVersion() string {
	if v := up.ready.Load(); v != nil {
		return *v
	}
	return ""
}

// stop takes the swap lock and keeps it, so a swap under way finishes and none
// starts while the program exits.
func (up *updater) stop() {
	up.swapping.Lock()
}
