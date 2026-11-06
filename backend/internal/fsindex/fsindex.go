// Package fsindex is what /api/v1 asks of the file index: folder sizes and
// entry counts for listings, and bringing a folder up to date after
// something changed inside it.
package fsindex

import (
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

// FolderPictures are the names media libraries give a folder's picture, in
// the order they are preferred.
var FolderPictures = []string{"poster.jpg", "folder.jpg", "cover.jpg"}

// Folder is what the index knows about one folder.
type Folder struct {
	// Items counts the entries directly inside.
	Items int
	// Size is the total size of everything below, or -1 while the index
	// has not measured it.
	Size int64
	// Pictures holds the files inside that are named like a folder picture.
	Pictures []string
}

// Index is the file index as the API sees it. Paths are index paths inside
// a source, such as "/filme/Sintel (2010)".
type Index interface {
	// Folders looks up the given folders. Folders the index has not seen
	// are missing from the result.
	Folders(src *ccfs.Source, dirs []string) map[string]Folder
	// FolderSize returns the total size of everything below dir, if the
	// index has measured it. It costs no query, so it can be asked per entry.
	FolderSize(src *ccfs.Source, dir string) (int64, bool)
	// Totals counts the files and folders anywhere below dir. ok is false
	// when the index holds nothing there.
	Totals(src *ccfs.Source, dir string) (files, dirs int64, ok bool)
	// Refresh reads dir again after something changed inside it, and a
	// folder inside that it has not seen before with everything below.
	Refresh(src *ccfs.Source, dir string)
	// Created brings in a new file or folder at p, a folder with
	// everything inside.
	Created(src *ccfs.Source, p string)
	// Moved follows an entry from one path to another inside src, so a
	// renamed folder is not read again.
	Moved(src *ccfs.Source, from, to string)
	// Removed drops the entry at p and everything below it.
	Removed(src *ccfs.Source, p string)
}

// Scanned returns the index the indexing package builds for each source.
func Scanned() Index { return scanned{} }

type scanned struct{}

func (scanned) Folders(src *ccfs.Source, dirs []string) map[string]Folder {
	idx := indexing.Lookup(src.Name)
	db := indexing.GetIndexDB()
	if idx == nil || db == nil || len(dirs) == 0 {
		return nil
	}
	keys := make([]string, len(dirs))
	for i, d := range dirs {
		keys[i] = slashed(d)
	}
	counts, err := db.FolderCounts(src.Name, keys)
	if err != nil {
		logger.Debugf("folder counts for %s: %v", src.Name, err)
		return nil
	}
	pics, err := db.FilesNamed(src.Name, keys, FolderPictures)
	if err != nil {
		logger.Debugf("folder pictures for %s: %v", src.Name, err)
	}
	out := make(map[string]Folder, len(counts))
	for i, d := range dirs {
		n, seen := counts[keys[i]]
		size, sized := idx.GetFolderSizeForIndexPath(keys[i])
		if !seen && !sized {
			continue
		}
		f := Folder{Items: n, Size: -1, Pictures: pics[keys[i]]}
		if sized {
			f.Size = int64(size)
		}
		out[d] = f
	}
	return out
}

func (scanned) FolderSize(src *ccfs.Source, dir string) (int64, bool) {
	idx := indexing.Lookup(src.Name)
	if idx == nil {
		return 0, false
	}
	size, ok := idx.GetFolderSizeForIndexPath(slashed(dir))
	return int64(size), ok
}

func (scanned) Totals(src *ccfs.Source, dir string) (files, dirs int64, ok bool) {
	db := indexing.GetIndexDB()
	if indexing.Lookup(src.Name) == nil || db == nil {
		return 0, 0, false
	}
	files, dirs, err := db.TreeCounts(src.Name, slashed(dir))
	if err != nil {
		logger.Debugf("tree counts for %s%s: %v", src.Name, dir, err)
		return 0, 0, false
	}
	return files, dirs, files+dirs > 0
}

func (scanned) Refresh(src *ccfs.Source, dir string) {
	if idx := indexing.Lookup(src.Name); idx != nil {
		idx.RefreshDirectoryAsync(slashed(dir))
	}
}

func (scanned) Created(src *ccfs.Source, p string) {
	if idx := indexing.Lookup(src.Name); idx != nil && p != "/" {
		idx.CreatedAsync(p)
	}
}

func (scanned) Moved(src *ccfs.Source, from, to string) {
	if idx := indexing.Lookup(src.Name); idx != nil && from != "/" && to != "/" {
		idx.MovedAsync(from, to)
	}
}

func (scanned) Removed(src *ccfs.Source, p string) {
	if idx := indexing.Lookup(src.Name); idx != nil && p != "/" {
		idx.RemovedAsync(p)
	}
}

// Follow keeps ix up to date with the changes the app makes itself, as
// fschange reports them, whether or not anyone has the folder open. It
// returns a function that stops it.
func Follow(fs *ccfs.Resolver, ix Index) (stop func()) {
	at := func(p string) (ccfs.Location, bool) {
		loc, err := fs.Resolve(p)
		return loc, err == nil && !loc.Virtual() && !loc.InArchive()
	}
	return fschange.Observe(fschange.Observer{
		Created: func(p string) {
			if loc, ok := at(p); ok {
				ix.Created(loc.Source, loc.Index)
			}
		},
		Removed: func(p string) {
			if loc, ok := at(p); ok {
				ix.Removed(loc.Source, loc.Index)
			}
		},
		Moved: func(from, to string) {
			src, okFrom := at(from)
			dst, okTo := at(to)
			if okFrom && okTo && src.Source.Name == dst.Source.Name {
				ix.Moved(src.Source, src.Index, dst.Index)
				return
			}
			if okFrom {
				ix.Removed(src.Source, src.Index)
			}
			if okTo {
				ix.Created(dst.Source, dst.Index)
			}
		},
	})
}

// slashed writes a folder the way the index stores it, with a trailing slash.
func slashed(dir string) string {
	if strings.HasSuffix(dir, "/") {
		return dir
	}
	return dir + "/"
}

// Inodes answers hard link lookups for the unraid package from the index of
// the source mounted at /mnt/<device>. Only disks and pools that are sources
// of their own are indexed with the inode numbers of the disk, since
// /mnt/user reports numbers of its own.
func Inodes(fs *ccfs.Resolver) InodeLookup { return InodeLookup{fs} }

// InodeLookup is the hard link lookup Inodes returns.
type InodeLookup struct{ fs *ccfs.Resolver }

// PathsByInode returns the other names below /mnt/<device>, without the
// leading slash. ok is false when the index cannot answer, so the caller
// walks the disk instead.
func (i InodeLookup) PathsByInode(device string, ino uint64) ([]string, bool) {
	db := indexing.GetIndexDB()
	if db == nil {
		return nil, false
	}
	for _, s := range i.fs.Sources() {
		if s.Mount != "/mnt/"+device || indexing.Lookup(s.Name) == nil {
			continue
		}
		paths, err := db.PathsByInode(s.Name, ino)
		if err != nil || len(paths) == 0 {
			return nil, false
		}
		for k, p := range paths {
			paths[k] = strings.TrimPrefix(p, "/")
		}
		return paths, true
	}
	return nil, false
}
