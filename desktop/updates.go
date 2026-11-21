package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/cmd"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/appimage"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/flatpak"
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

	// mu guards the update started by hand.
	mu      sync.Mutex
	busy    bool
	percent int
	failed  string
}

func newUpdater(logger *log.Logger) *updater {
	u := &update.Updater{
		Repo:    update.Repo,
		Version: cmd.Version(),
		Assets:  update.Assets,
		API:     updateAPI,
		// Long enough for the zip on a slow line, short enough that a
		// connection that stalls does not hold up every later check.
		Client: &http.Client{Timeout: 30 * time.Minute},
		Logf:   logger.Printf,
	}
	if image := appimage.Path(); image != "" {
		u.ForAppImage(image)
	}
	up := &updater{log: logger, u: u, percent: -1}
	u.Progress = up.progress
	return up
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
// it is never checks, and a Flatpak leaves updates to the software centre,
// which owns its files.
func (up *updater) run(ctx context.Context, autoUpdate func() bool) {
	if flatpak.ID() != "" {
		up.log.Printf("update: the software centre keeps this Flatpak current")
		return
	}
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
			if err := up.once(ctx); err != nil {
				up.log.Printf("update: %v", err)
			}
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

// once updates to the newest release, if there is a newer one.
func (up *updater) once(ctx context.Context) error {
	rel, err := up.u.Check(ctx)
	if err != nil {
		return err
	}
	if rel == nil {
		up.log.Printf("update: no newer release")
		return nil
	}
	download, err := up.u.Fetch(ctx, rel)
	if err != nil {
		return fmt.Errorf("not updating to %s: %w", rel.Version, err)
	}
	defer os.Remove(download)

	up.swapping.Lock()
	err = up.u.Swap(download, rel)
	up.swapping.Unlock()
	if err != nil {
		return fmt.Errorf("not updating to %s: %w", rel.Version, err)
	}
	up.log.Printf("update: %s is in place and starts next time", rel.Version)
	up.ready.Store(&rel.Version)
	return nil
}

func (up *updater) progress(done, total int64) {
	percent := -1
	if total > 0 {
		percent = int(done * 100 / total)
	}
	up.mu.Lock()
	up.percent = percent
	up.mu.Unlock()
}

// state is how the update started by hand goes.
func (up *updater) state() desktop.UpdateState {
	up.mu.Lock()
	defer up.mu.Unlock()
	return desktop.UpdateState{Busy: up.busy, Percent: up.percent, Ready: up.readyVersion(), Failed: up.failed}
}

// now is "Update now". The installed copy cannot write to its own folder, so
// it runs the scheduled task's update with administrator rights and then
// reads the new version from the uninstall entry.
func (up *updater) now(installed bool) {
	up.mu.Lock()
	if up.busy {
		up.mu.Unlock()
		return
	}
	up.busy, up.percent, up.failed = true, -1, ""
	up.mu.Unlock()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		var err error
		if installed {
			if err = updateElevated(); err == nil {
				if v := update.InstalledVersion(product); update.Newer(v, cmd.Version()) {
					up.ready.Store(&v)
				}
			}
		} else {
			err = up.once(ctx)
		}
		if err != nil {
			up.log.Printf("update: %v", err)
		}
		up.mu.Lock()
		up.busy = false
		if err != nil {
			up.failed = err.Error()
		}
		up.mu.Unlock()
	}()
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

// shellKnows tells the shell once per version, from the person's own session,
// that the program changed, so shortcuts drop the icon they cached for an
// earlier one. The scheduled task that updates the installed copy runs as the
// system account, and what it tells the shell never reaches the desktop. mark
// keeps the version last told.
func shellKnows(mark string) {
	if data, err := os.ReadFile(mark); err == nil && strings.TrimSpace(string(data)) == cmd.Version() {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	update.ShellChanged(exe)
	_ = os.WriteFile(mark, []byte(cmd.Version()), 0o644)
}
