package remotes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
)

// Create adds a remote and mounts it.
func (s *Service) Create(ctx context.Context, in Input) (Remote, error) {
	return s.create(ctx, in, true)
}

// create adds a remote, mounted right away or left for later.
func (s *Service) create(ctx context.Context, in Input, mount bool) (Remote, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return Remote{}, err
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	name := strings.TrimSpace(in.Name)
	if err := s.checkName(name); err != nil {
		return Remote{}, err
	}
	rows, err := s.db.all(ctx)
	if err != nil {
		return Remote{}, err
	}
	if slices.ContainsFunc(rows, func(r row) bool { return r.Name == name }) {
		return Remote{}, v1.Errorf(http.StatusConflict, "there is already a remote called "+name)
	}
	p, err := s.provider(ctx, in.Type)
	if err != nil {
		return Remote{}, err
	}
	r := row{Name: name, Type: in.Type, Mount: strings.TrimSpace(in.Mount), MountOnStart: mount}
	if in.Path != nil {
		r.Path = strings.TrimSpace(*in.Path)
	}
	if r.Mount == "" {
		r.Mount = filepath.Join(s.cfg.Root, name)
	}
	r.Mount = filepath.Clean(r.Mount)
	if err := s.checkMount(r.Mount, "", rows); err != nil {
		return Remote{}, err
	}
	set, _, token, err := s.params(ctx, p, in.Params)
	if err != nil {
		return Remote{}, err
	}
	if err := s.createSection(ctx, name, r.Type, set, token); err != nil {
		s.forget(name)
		return Remote{}, err
	}
	if err := s.db.insert(ctx, r); err != nil {
		s.forget(name)
		return Remote{}, err
	}
	if mount {
		if err := s.mountRow(ctx, r, append(rows, r)); err != nil {
			logger.Errorf("remotes: mount %s on %s: %v", name, r.Mount, err)
		}
	}
	s.refreshLabels(ctx)
	return s.Get(ctx, name, true)
}

// Update changes a remote. A mounted remote is unmounted for the change
// and mounted again afterwards.
func (s *Service) Update(ctx context.Context, name string, in Input) (Remote, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return Remote{}, err
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()

	old, err := s.row(ctx, name)
	if err != nil {
		return Remote{}, err
	}
	if in.Type != "" && in.Type != old.Type {
		return Remote{}, v1.Errorf(http.StatusBadRequest, "the type of a remote stays, add a new remote for another type")
	}
	next := old
	if n := strings.TrimSpace(in.Name); n != "" && n != name {
		if err := s.checkName(n); err != nil {
			return Remote{}, err
		}
		_, taken, err := s.db.get(ctx, n)
		if err != nil {
			return Remote{}, err
		}
		if taken {
			return Remote{}, v1.Errorf(http.StatusConflict, "there is already a remote called "+n)
		}
		next.Name = n
	}
	if in.Path != nil {
		next.Path = strings.TrimSpace(*in.Path)
	}
	if m := strings.TrimSpace(in.Mount); m != "" {
		next.Mount = filepath.Clean(m)
	}
	rows, err := s.db.all(ctx)
	if err != nil {
		return Remote{}, err
	}
	if err := s.checkMount(next.Mount, name, rows); err != nil {
		return Remote{}, err
	}
	p, err := s.provider(ctx, old.Type)
	if err != nil {
		return Remote{}, err
	}
	set, unset, token, err := s.params(ctx, p, in.Params)
	if err != nil {
		return Remote{}, err
	}
	stored, err := s.section(ctx, name)
	if err != nil {
		return Remote{}, err
	}
	if token == "" {
		token = stored["token"]
	}

	wasMounted := s.mountedPoints(ctx)[old.Mount]
	if wasMounted {
		if err := s.unmountRow(ctx, old); err != nil {
			return Remote{}, err
		}
	}
	if next.Name != name {
		full := map[string]string{}
		for k, v := range stored {
			if k != "type" && !slices.Contains(unset, k) {
				full[k] = v
			}
		}
		for k, v := range set {
			full[k] = v
		}
		if err := s.createSection(ctx, next.Name, next.Type, full, token); err != nil {
			s.forget(next.Name)
			return Remote{}, err
		}
		s.forget(name)
		s.noteError(name, nil)
	} else if err := s.updateSection(ctx, name, set, unset, token); err != nil {
		return Remote{}, err
	}
	if err := s.db.replace(ctx, name, next); err != nil {
		return Remote{}, err
	}
	if next.Mount != old.Mount {
		_ = os.Remove(old.Mount)
	}
	if wasMounted {
		if err := s.mountRow(ctx, next, rows); err != nil {
			logger.Errorf("remotes: mount %s on %s: %v", next.Name, next.Mount, err)
		}
	}
	s.refreshLabels(ctx)
	return s.Get(ctx, next.Name, true)
}

// Delete unmounts a remote and forgets it with its credentials. The files
// at the remote stay where they are.
func (s *Service) Delete(ctx context.Context, name string) error {
	if err := s.ready(ctx, readyWait); err != nil {
		return err
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.row(ctx, name)
	if err != nil {
		return err
	}
	if s.mountedPoints(ctx)[r.Mount] {
		if err := s.unmountRow(ctx, r); err != nil {
			return err
		}
	}
	if err := s.rc.call(ctx, "config/delete", map[string]string{"name": name}, nil); err != nil {
		return err
	}
	if err := s.db.remove(ctx, name); err != nil {
		return err
	}
	s.noteError(name, nil)
	// Only an empty folder goes, so files someone put there stay.
	_ = os.Remove(r.Mount)
	s.refreshLabels(ctx)
	return nil
}

// Mount mounts a remote and keeps it mounted across restarts.
func (s *Service) Mount(ctx context.Context, name string) (Remote, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return Remote{}, err
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.row(ctx, name)
	if err != nil {
		return Remote{}, err
	}
	if !s.mountedPoints(ctx)[r.Mount] {
		rows, err := s.db.all(ctx)
		if err != nil {
			return Remote{}, err
		}
		if err := s.mountRow(ctx, r, rows); err != nil {
			return Remote{}, v1.Errorf(http.StatusConflict, "could not mount "+name+": "+mountMessage(err))
		}
	}
	if err := s.db.setMountOnStart(ctx, name, true); err != nil {
		return Remote{}, err
	}
	return s.Get(ctx, name, true)
}

// Unmount unmounts a remote and leaves it unmounted across restarts.
func (s *Service) Unmount(ctx context.Context, name string) (Remote, error) {
	if err := s.ready(ctx, readyWait); err != nil {
		return Remote{}, err
	}
	ctx, cancel := detached(ctx)
	defer cancel()
	s.mu.Lock()
	defer s.mu.Unlock()
	r, err := s.row(ctx, name)
	if err != nil {
		return Remote{}, err
	}
	if s.mountedPoints(ctx)[r.Mount] {
		if err := s.unmountRow(ctx, r); err != nil {
			return Remote{}, err
		}
	}
	if err := s.db.setMountOnStart(ctx, name, false); err != nil {
		return Remote{}, err
	}
	s.noteError(name, nil)
	return s.Get(ctx, name, true)
}

func mountMessage(err error) string {
	var rc *rcError
	if errors.As(err, &rc) {
		return rc.Msg
	}
	return err.Error()
}

// cacheShare splits the cache cap over the remotes that are meant to be
// mounted, in MiB. rclone caps each mount on its own.
func cacheShare(gb int, rows []row) int {
	n := 0
	for _, r := range rows {
		if r.MountOnStart {
			n++
		}
	}
	return gb * 1024 / max(n, 1)
}

func (s *Service) mountRow(ctx context.Context, r row, rows []row) error {
	err := s.mountOn(ctx, r, rows)
	s.noteError(r.Name, err)
	return err
}

func (s *Service) mountOn(ctx context.Context, r row, rows []row) error {
	clearStale(r.Mount)
	if err := os.MkdirAll(r.Mount, 0o777); err != nil {
		return err
	}
	if entries, err := os.ReadDir(r.Mount); err != nil {
		return err
	} else if len(entries) > 0 {
		return fmt.Errorf("%s is not empty, rclone mounts on an empty folder only", r.Mount)
	}
	m, err := s.mounting(ctx)
	if err != nil {
		return err
	}
	vfs := map[string]any{"CacheMode": m.Cache}
	if m.Cache != system.CacheOff {
		vfs["CacheMaxSize"] = fmt.Sprintf("%dM", cacheShare(m.CacheSize, rows))
	}
	opts := map[string]any{"VolumeName": r.Name, "AllowOther": true}
	call := func() error {
		return s.rc.call(ctx, "mount/mount", map[string]any{"fs": r.fs(), "mountPoint": r.Mount, "vfsOpt": vfs, "mountOpt": opts}, nil)
	}
	err = call()
	// Other containers, a media server say, only see the files with
	// allow_other, which FUSE grants a user that is not root only when
	// fuse.conf says so. Mounted without it the files are still here.
	if err != nil && strings.Contains(err.Error(), "allow_other") {
		opts["AllowOther"] = false
		err = call()
	}
	return err
}

func (s *Service) unmountRow(ctx context.Context, r row) error {
	if err := s.rc.call(ctx, "mount/unmount", map[string]string{"mountPoint": r.Mount}, nil); err != nil {
		return v1.Errorf(http.StatusConflict, "could not unmount "+r.Name+", a program may still have files open there: "+mountMessage(err))
	}
	return nil
}
