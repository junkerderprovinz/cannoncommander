package modules

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/desktop/update"
)

// Folder is where the program at exe keeps its modules: a modules folder
// beside it, or on macOS one per processor inside the bundle, which the
// build fills and an update of the bundle replaces.
func Folder(exe string) string {
	if runtime.GOOS == "darwin" {
		return filepath.Join(filepath.Dir(filepath.Dir(exe)), "Resources", "modules", runtime.GOARCH)
	}
	return filepath.Join(filepath.Dir(exe), "modules")
}

// Read returns the record of the modules in dir, which must be for this
// platform.
func Read(dir string) (Record, error) {
	var rec Record
	data, err := os.ReadFile(filepath.Join(dir, RecordFile))
	if err != nil {
		return rec, err
	}
	if err := json.Unmarshal(data, &rec); err != nil {
		return rec, fmt.Errorf("%s: %w", RecordFile, err)
	}
	if want := runtime.GOOS + "/" + runtime.GOARCH; rec.Platform != want {
		return rec, fmt.Errorf("the modules in %s are for %s, not %s", dir, rec.Platform, want)
	}
	return rec, nil
}

// Use puts the modules of rec, found in dir, first on the PATH, so the
// server and every program it starts find them, and points the variables
// they name at them unless those are set already.
func Use(dir string, rec Record) {
	var paths []string
	for _, name := range slices.Sorted(maps.Keys(rec.Modules)) {
		m := rec.Modules[name]
		root := filepath.Join(dir, name)
		paths = append(paths, filepath.Join(root, filepath.FromSlash(m.Path)))
		for k, v := range m.Env {
			if os.Getenv(k) == "" {
				os.Setenv(k, filepath.Join(root, filepath.FromSlash(v)))
			}
		}
	}
	os.Setenv("PATH", strings.Join(append(paths, os.Getenv("PATH")), string(os.PathListSeparator)))
}

// Installer puts the modules of a release in place of the ones in Dir.
type Installer struct {
	// Updater reaches the releases, as for an update of the program.
	Updater *update.Updater
	Dir     string
	// Report hears how far the install has come, from 0 to 1, and what it
	// does. It may be nil.
	Report func(fraction float64, status string)

	last float64
}

// ErrNextStart means the modules are in place for the next start: Windows
// kept the old folder, since a program in it was running.
var ErrNextStart = errors.New("the modules are ready for the next start")

// Install downloads the modules zip of the release tagged tag beside Dir,
// checks it against the release's checksums.txt, unpacks it and swaps the
// result for Dir, so the folder is never half replaced.
func (in *Installer) Install(ctx context.Context, tag string) error {
	parent := filepath.Dir(in.Dir)
	in.report(0, "Downloading the modules")
	zip, err := in.Updater.FetchFile(ctx, tag, AssetName(runtime.GOOS, runtime.GOARCH), parent, func(done, total int64) {
		if total > 0 {
			in.report(0.9*float64(done)/float64(total), "")
		}
	})
	if err != nil {
		return err
	}
	defer os.Remove(zip)

	in.report(0.9, "Unpacking")
	staged, err := os.MkdirTemp(parent, stagePrefix(in.Dir))
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	if err := update.Unzip(zip, staged); err != nil {
		return err
	}
	rec, err := Read(staged)
	if err != nil {
		return err
	}
	rec.Release = tag
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(staged, RecordFile), append(data, '\n'), 0o644); err != nil {
		return err
	}

	in.report(0.98, "Putting them in place")
	err = swap(staged, in.Dir)
	if errors.Is(err, errBusy) {
		next := in.Dir + ".next"
		if err := os.RemoveAll(next); err != nil {
			return err
		}
		if err := update.Rename(staged, next); err != nil {
			return err
		}
		in.report(1, "Ready for the next start")
		return ErrNextStart
	}
	if err != nil {
		return err
	}
	in.report(1, "Done")
	return nil
}

// report passes on progress in steps of a hundredth, and every status.
func (in *Installer) report(fraction float64, status string) {
	if in.Report == nil || (status == "" && fraction-in.last < 0.01) {
		return
	}
	in.last = fraction
	in.Report(fraction, status)
}

// Prepare finishes what an earlier install left in dir's folder: modules
// that waited for this start, and the files of an install that never ended.
func Prepare(dir string) error {
	parent := filepath.Dir(dir)
	leftovers, _ := filepath.Glob(filepath.Join(parent, stagePrefix(dir)+"*"))
	downloads, _ := filepath.Glob(filepath.Join(parent, "."+AssetName(runtime.GOOS, runtime.GOARCH)+".download-*"))
	var errs []error
	for _, p := range append(append(leftovers, downloads...), dir+".old") {
		if err := os.RemoveAll(p); err != nil {
			errs = append(errs, err)
		}
	}
	next := dir + ".next"
	if _, err := os.Stat(filepath.Join(next, RecordFile)); err == nil {
		errs = append(errs, swap(next, dir))
	}
	return errors.Join(errs...)
}

var errBusy = errors.New("the modules folder is in use")

// swap puts the folder staged in place of dir. The old dir steps aside as
// .old first, which Windows refuses while a program in it runs.
func swap(staged, dir string) error {
	old := dir + ".old"
	if err := os.RemoveAll(old); err != nil {
		return err
	}
	_, err := os.Lstat(dir)
	switch {
	case err == nil:
		if err := update.Rename(dir, old); err != nil {
			return fmt.Errorf("%w: %v", errBusy, err)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return err
	}
	if err := update.Rename(staged, dir); err != nil {
		update.Rename(old, dir)
		return err
	}
	// Whatever of the old folder stays goes with the next Prepare.
	os.RemoveAll(old)
	return nil
}

// stagePrefix names the folders an install unpacks into beside dir.
func stagePrefix(dir string) string {
	return "." + filepath.Base(dir) + ".new-"
}
