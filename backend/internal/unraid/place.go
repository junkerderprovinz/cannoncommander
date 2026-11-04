package unraid

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

// maxSurvey bounds a walk over a folder, so a request on a share with
// millions of files answers with a partial count instead of hanging.
const maxSurvey = 200_000

// Place is where an API path lies on Unraid. Vol is user, user0 or the name
// of a disk or pool, Rel the path below /mnt/<vol> and Share its first part.
type Place struct {
	Vol   string
	Rel   string
	Share string
}

// User reports whether the place is in the merged user share view.
func (p Place) User() bool { return p.Vol == "user" || p.Vol == "user0" }

// placeOf maps a clean API path to its place. ok is false for paths that
// are not on the array or a pool, such as /mnt/remotes or /boot.
func placeOf(st *State, apiPath string) (Place, bool) {
	rest, ok := strings.CutPrefix(apiPath, "/mnt/")
	if !ok || strings.Contains(apiPath, ccfs.ArchiveSep) {
		return Place{}, false
	}
	vol, rel, _ := strings.Cut(rest, "/")
	if vol != "user" && vol != "user0" && !slices.Contains(st.Devices(), vol) {
		return Place{}, false
	}
	share, _, _ := strings.Cut(rel, "/")
	return Place{Vol: vol, Rel: rel, Share: share}, true
}

// file is one file as it lies on a disk or pool. rel is below /mnt/<dev>.
type file struct {
	rel  string
	dev  string
	size int64
	ino  inode
}

func (f file) share() string {
	s, _, _ := strings.Cut(f.rel, "/")
	return s
}

type inode struct {
	dev, ino, nlink uint64
}

// devicesFor returns the disks and pools a place can have files on. user0
// is the array without the pools.
func devicesFor(st *State, p Place) []string {
	switch p.Vol {
	case "user":
		return st.Devices()
	case "user0":
		return st.DataDisks()
	}
	return []string{p.Vol}
}

func (s *Service) real(dev, rel string) string {
	return filepath.Join(s.cfg.Mnt, dev, filepath.FromSlash(rel))
}

// survey lists every file below p on the devices that hold it. A path in a
// user share is looked up on each disk and pool, because only there do
// sizes, inodes and link counts belong to the real file.
func (s *Service) survey(ctx context.Context, st *State, p Place) (files []file, partial bool) {
	for _, dev := range devicesFor(st, p) {
		base := filepath.Join(s.cfg.Mnt, dev)
		root := s.real(dev, p.Rel)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if len(files) >= maxSurvey || ctx.Err() != nil {
				partial = true
				return filepath.SkipAll
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(base, path)
			files = append(files, file{rel: filepath.ToSlash(rel), dev: dev, size: info.Size(), ino: inodeOf(info)})
			return nil
		})
	}
	return files, partial
}

// DevicesOf returns the disks and pools that hold apiPath, without
// walking into a folder. It is cheap enough for a listing column.
func (s *Service) DevicesOf(apiPath string) []string {
	st, err := s.State()
	if err != nil {
		return nil
	}
	p, ok := placeOf(st, apiPath)
	if !ok {
		return nil
	}
	if !p.User() {
		return []string{p.Vol}
	}
	var out []string
	for _, dev := range devicesFor(st, p) {
		if _, err := os.Lstat(s.real(dev, p.Rel)); err == nil {
			out = append(out, dev)
		}
	}
	return out
}

// otherNames finds the other paths of a hard link on its device, as paths
// below /mnt/<dev>. complete is false when the walk ran out of time before
// every name turned up.
func (s *Service) otherNames(ctx context.Context, f file) (rels []string, complete bool) {
	want := int(f.ino.nlink) - 1
	if want <= 0 {
		return nil, true
	}
	s.mu.Lock()
	ix := s.index
	s.mu.Unlock()
	if ix != nil {
		if found, ok := ix.PathsByInode(f.dev, f.ino.ino); ok {
			for _, r := range found {
				if r != f.rel {
					rels = append(rels, r)
				}
			}
			return rels, len(rels) >= want
		}
	}
	ctx, cancel := context.WithTimeout(ctx, s.cfg.LinkBudget)
	defer cancel()
	base := filepath.Join(s.cfg.Mnt, f.dev)
	_ = filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return filepath.SkipAll
		}
		if err != nil || !d.Type().IsRegular() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if in := inodeOf(info); in.ino != f.ino.ino || in.dev != f.ino.dev {
			return nil
		}
		rel, _ := filepath.Rel(base, path)
		if rel = filepath.ToSlash(rel); rel != f.rel {
			rels = append(rels, rel)
		}
		if len(rels) >= want {
			return filepath.SkipAll
		}
		return nil
	})
	return rels, len(rels) >= want
}

// shownPath is the path a person knows a file by: the user share when the
// file belongs to one, the disk otherwise.
func shownPath(st *State, dev, rel string) string {
	share, _, _ := strings.Cut(rel, "/")
	if _, ok := st.Share(share); ok {
		return "/mnt/user/" + rel
	}
	return "/mnt/" + dev + "/" + rel
}
