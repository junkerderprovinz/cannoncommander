package fileops

import (
	"context"
	"os"
	"path"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fschange"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
)

// partAge is how long a part file stays without a write before it counts as
// left behind. A job that comes back to a missing part file starts that file
// over, so a wrong guess costs time and no data.
const partAge = 24 * time.Hour

// sweepParts removes the part files a crash left behind. It finds them through
// the index, because looking on disk would wake every disk of the array.
func (s *Service) sweepParts(ctx context.Context) {
	rows, err := s.uploadRows(ctx, "")
	if err != nil {
		logger.Errorf("part files: %v", err)
		return
	}
	open := make(map[string]bool, len(rows))
	for _, row := range rows {
		open[partPath(row.path)] = true
	}
	cutoff := time.Now().Add(-partAge)
	for _, src := range s.fs.Sources() {
		paths, err := fsindex.Parts(ctx, &src)
		if err != nil {
			logger.Errorf("part files of %s: %v", src.Name, err)
			continue
		}
		for _, p := range paths {
			if ctx.Err() != nil {
				return
			}
			if !open[p] {
				s.dropPart(p, cutoff)
			}
		}
	}
}

func partPath(target string) string {
	return path.Join(path.Dir(target), "."+path.Base(target)+PartSuffix)
}

// dropPart looks at the file itself, since the index can be behind.
func (s *Service) dropPart(p string, cutoff time.Time) {
	loc, err := s.fs.Resolve(p)
	if err != nil || ccfs.Contained(loc) != nil {
		return
	}
	info, err := os.Lstat(loc.Real)
	if err != nil || !info.Mode().IsRegular() || info.ModTime().After(cutoff) {
		return
	}
	if err := os.Remove(loc.Real); err != nil {
		logger.Errorf("part file %s: %v", p, err)
		return
	}
	fschange.Removed(p)
}
