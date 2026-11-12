package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/cmd"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/flatpak"
	"github.com/junkerderprovinz/cannoncommander/desktop/modules"
	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

var installModules = flag.Bool("modules", false, "install the helper programs of this version beside the program and exit, saying how it goes on stdout as the installer reads it")

// helpers keeps this copy's modules, the helper programs that come in a zip
// of their own with each release. See desktop/modules.
type helpers struct {
	folder   string
	logger   *log.Logger
	releases *update.Updater
	// installed marks the copy installed for all users, whose folder only an
	// administrator may write.
	installed bool

	mu       sync.Mutex
	progress desktop.ModulesState
}

var _ desktop.Modules = (*helpers)(nil)

func newHelpers(logger *log.Logger, up *update.Updater, installed bool) (*helpers, error) {
	dir, err := moduleFolder()
	if err != nil {
		return nil, err
	}
	h := &helpers{folder: dir, logger: logger, releases: up, installed: installed}
	h.progress.Offered = fetchesModules()
	return h, nil
}

func moduleFolder() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return modules.Folder(exe), nil
}

// fetchesModules reports whether this copy fetches its modules. A Mac bundle
// carries them and a Flatpak brings its own, and a build that is no release
// has no release to take them from.
func fetchesModules() bool {
	return runtime.GOOS != "darwin" && flatpak.ID() == "" && update.IsRelease(cmd.Version())
}

// use finishes an earlier install and puts the modules on the PATH, before
// the server starts and looks for its programs.
func (h *helpers) use() {
	if err := modules.Prepare(h.folder); err != nil {
		h.logger.Printf("modules: %v", err)
	}
	rec, err := modules.Read(h.folder)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			h.logger.Printf("modules: %v", err)
		}
		return
	}
	modules.Use(h.folder, rec)
}

// catchUp fetches the modules when they are missing or older than this
// program, as after the program updated itself. The installed
// copy leaves that to the installer and its scheduled task.
func (h *helpers) catchUp(ctx context.Context) {
	if !h.progress.Offered || h.installed || !stale(h.folder, cmd.Version()) {
		return
	}
	h.fetch(ctx)
}

// stale reports whether the modules in dir are missing or older than
// version.
func stale(dir, version string) bool {
	rec, err := modules.Read(dir)
	return err != nil || update.Newer(version, rec.Release)
}

func (h *helpers) ModulesState() desktop.ModulesState {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.progress
}

func (h *helpers) FetchModules() error {
	if !h.progress.Offered {
		return errors.ErrUnsupported
	}
	go h.fetch(context.Background())
	return nil
}

func (h *helpers) fetch(ctx context.Context) {
	h.mu.Lock()
	if h.progress.Running {
		h.mu.Unlock()
		return
	}
	h.progress.Running, h.progress.Failed, h.progress.Fraction = true, false, 0
	h.mu.Unlock()

	var err error
	if h.installed {
		err = fetchElevated()
	} else {
		in := &modules.Installer{Updater: h.releases, Dir: h.folder, Report: func(f float64, _ string) {
			h.mu.Lock()
			h.progress.Fraction = f
			h.mu.Unlock()
		}}
		err = in.Install(ctx, cmd.Version())
	}
	ready := err == nil || errors.Is(err, modules.ErrNextStart)
	if err == nil {
		if rec, rerr := modules.Read(h.folder); rerr == nil {
			modules.Use(h.folder, rec)
		}
	}
	if ready {
		h.logger.Printf("modules: %s is in place", cmd.Version())
	} else {
		h.logger.Printf("modules: %v", err)
	}
	h.mu.Lock()
	h.progress.Running, h.progress.Ready, h.progress.Failed = false, ready, !ready
	if ready {
		h.progress.Fraction = 1
	}
	h.mu.Unlock()
}

// refreshModules is --modules, the installer's step: it installs the modules
// of this version beside the program and says how it goes on stdout.
func refreshModules() error {
	report := modules.PrintProgress(os.Stdout)
	if runtime.GOOS == "darwin" {
		report(1, "The helper programs come inside the app")
		return nil
	}
	if !update.IsRelease(cmd.Version()) {
		return fmt.Errorf("%s is not a release build, so there is no release to take the modules from", cmd.Version())
	}
	dir, err := moduleFolder()
	if err != nil {
		return err
	}
	if err := modules.Prepare(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	up := newUpdater(log.New(os.Stderr, "", log.LstdFlags))
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	err = (&modules.Installer{Updater: up.u, Dir: dir, Report: report}).Install(ctx, cmd.Version())
	if errors.Is(err, modules.ErrNextStart) {
		return nil
	}
	return err
}

// refreshInstalled brings the installed copy's modules to version, the
// release the scheduled task has just put in place or the running one.
func refreshInstalled(ctx context.Context, logger *log.Logger, up *update.Updater, version string) {
	dir, err := moduleFolder()
	if err != nil {
		logger.Printf("modules: %v", err)
		return
	}
	if err := modules.Prepare(dir); err != nil {
		logger.Printf("modules: %v", err)
	}
	if !stale(dir, version) {
		return
	}
	err = (&modules.Installer{Updater: up, Dir: dir}).Install(ctx, version)
	switch {
	case errors.Is(err, modules.ErrNextStart):
		logger.Printf("modules: %s waits for the programs of the old ones to end", version)
	case err != nil:
		logger.Printf("modules: not updating to %s: %v", version, err)
	default:
		logger.Printf("modules: %s is in place", version)
	}
}
