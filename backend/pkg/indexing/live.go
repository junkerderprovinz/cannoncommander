package indexing

import (
	"os"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
)

// CreatedAsync brings a new entry at indexPath into the index in the
// background: a folder with everything inside, then the folder holding it.
func (idx *Index) CreatedAsync(indexPath string) {
	p := strings.TrimSuffix(indexPath, "/")
	idx.bgWork.submit("created|"+p, func() {
		idx.Created(p)
		idx.RefreshDirectoryAsync(parentFolder(p))
	})
}

// MovedAsync follows an entry from one index path to another in the
// background and reads both folders around it again.
func (idx *Index) MovedAsync(from, to string) {
	from, to = strings.TrimSuffix(from, "/"), strings.TrimSuffix(to, "/")
	idx.bgWork.submit("moved|"+from+"|"+to, func() {
		idx.Moved(from, to)
		idx.RefreshDirectoryAsync(parentFolder(from))
		if parentFolder(to) != parentFolder(from) {
			idx.RefreshDirectoryAsync(parentFolder(to))
		}
	})
}

// RemovedAsync drops the entry at indexPath with everything below by
// reading the folder that held it again.
func (idx *Index) RemovedAsync(indexPath string) {
	idx.RefreshDirectoryAsync(parentFolder(strings.TrimSuffix(indexPath, "/")))
}

// Created reads the folder at indexPath with everything inside. Files are
// left to the refresh of the folder holding them.
func (idx *Index) Created(indexPath string) {
	p := strings.TrimSuffix(indexPath, "/")
	info, err := os.Stat(utils.JoinUnderSourceRoot(idx.Path, p))
	if err != nil || !info.IsDir() {
		return
	}
	if err := idx.RefreshDirectory(p+"/", true); err != nil {
		logger.Debugf("Failed to read new folder %s: %v", p, err)
	}
}

// Moved gives the rows and folder sizes of the entry at from, and of
// everything below it, the paths they have under to. An entry moved where
// the index does not reach, such as into the trash, leaves the index.
func (idx *Index) Moved(from, to string) {
	from, to = strings.TrimSuffix(from, "/"), strings.TrimSuffix(to, "/")
	real := utils.JoinUnderSourceRoot(idx.Path, to)
	info, err := os.Lstat(real)
	if err != nil || from == to {
		return
	}
	if info.IsDir() {
		from, to = from+"/", to+"/"
	}
	if IsHidden(real) || !idx.holds(parentFolder(to)) {
		idx.dropTrees([]string{from})
		return
	}
	moved, err := idx.db.MoveTree(idx.Name, from, to)
	if err != nil {
		logger.Debugf("Failed to move index rows from %s to %s: %v", from, to, err)
		return
	}
	if !moved || !info.IsDir() {
		return
	}
	idx.folderSizesMu.Lock()
	defer idx.folderSizesMu.Unlock()
	sizes := map[string]uint64{}
	for key, size := range idx.folderSizes {
		if rest, ok := strings.CutPrefix(key, from); ok {
			sizes[to+rest] = size
			delete(idx.folderSizes, key)
			delete(idx.folderSizesUnsynced, key)
		}
	}
	for key, size := range sizes {
		idx.folderSizes[key] = size
		idx.folderSizesUnsynced[key] = struct{}{}
	}
}

// dropGone removes the rows of the entries in known that the listing of dir
// no longer holds, and returns the listed folders known lacks.
func (idx *Index) dropGone(dir string, known map[string]bool, listing *iteminfo.FileInfo) []string {
	listed := make(map[string]bool, len(listing.Files)+len(listing.Folders))
	var fresh []string
	for _, f := range listing.Folders {
		p := dir + f.Name + "/"
		listed[p] = true
		if _, ok := known[p]; !ok {
			fresh = append(fresh, p)
		}
	}
	for _, f := range listing.Files {
		listed[dir+f.Name] = true
	}
	var gone []string
	for p := range known {
		if !listed[p] {
			gone = append(gone, p)
		}
	}
	idx.dropTrees(gone)
	return fresh
}

// dropTrees removes the rows of paths and everything below them, and the
// sizes of those folders. The folders above keep their sizes until their
// next refresh, which counts what is left.
func (idx *Index) dropTrees(paths []string) {
	if len(paths) == 0 {
		return
	}
	if err := idx.db.DeleteTrees(idx.Name, paths); err != nil {
		logger.Debugf("Failed to drop %d index entries: %v", len(paths), err)
	}
	gone := map[string]bool{}
	for _, p := range paths {
		if strings.HasSuffix(p, "/") {
			gone[p] = true
		}
	}
	if len(gone) == 0 {
		return
	}
	idx.folderSizesMu.Lock()
	defer idx.folderSizesMu.Unlock()
	for key := range idx.folderSizes {
		for k := key; k != ""; k = parentFolder(k) {
			if gone[k] {
				delete(idx.folderSizes, key)
				delete(idx.folderSizesUnsynced, key)
				break
			}
		}
	}
}

// holds reports whether the index has a row for the folder dir.
func (idx *Index) holds(dir string) bool {
	if dir == "/" {
		return true
	}
	item, err := idx.db.GetItem(idx.Name, utils.AddTrailingSlashIfNotExists(dir))
	return err == nil && item != nil
}

// readsNewFolders reports whether a refresh may read a folder it meets for
// the first time with everything inside. Before every scanner has been
// through once, the scanners are on their way there anyway.
func (idx *Index) readsNewFolders() bool {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	for _, s := range idx.scanners {
		var scanned bool
		s.withStatsRLock(func() { scanned = !s.lastScanned.IsZero() })
		if !scanned {
			return false
		}
	}
	return true
}

// parentFolder returns the index path of the folder holding p, with a
// trailing slash, or "" for the root.
func parentFolder(p string) string {
	p = strings.TrimSuffix(p, "/")
	if p == "" {
		return ""
	}
	return p[:strings.LastIndex(p, "/")+1]
}
