package fileops

import (
	"context"

	"github.com/gtsteffaniak/go-logger/logger"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

// PartSuffix marks a file that is still being written. The copy goes to
// ".name.cc-part" next to the target and only takes the real name once it
// is complete, so a half copied file never looks finished.
const PartSuffix = ccfs.PartSuffix

// Space is the plain free space at the target of a transfer.
type Space struct {
	Free   int64 `json:"free"`
	Need   int64 `json:"need"`
	Enough bool  `json:"enough"`
}

// Service holds what the handlers and job kinds of this package share.
type Service struct {
	deps   v1.Deps
	fs     *ccfs.Resolver
	trash  *trash.Store
	undo   *undo.Log
	jobs   *jobs.Manager
	unraid *unraid.Service
}

// For returns the service for the database in d, or nil without a
// database or resolver. Building it connects the job queue to Unraid.
func For(d v1.Deps) *Service {
	if d.DB == nil || d.FS == nil {
		return nil
	}
	s, _ := v1.Shared(d, "fileops", func() (*Service, error) {
		s := &Service{deps: d, fs: d.FS, trash: trash.For(d), undo: undo.For(d), jobs: jobs.For(d), unraid: unraid.Default()}
		if s.jobs != nil {
			connectUnraid(s.jobs, s.unraid)
		}
		if s.trash != nil {
			s.trash.AfterExpire(s.sweepParts)
		}
		return s, nil
	})
	return s
}

func plainSpace(dest ccfs.Location, need int64) *Space {
	free, ok := freeSpace(dest.Real)
	if !ok {
		return nil
	}
	return &Space{Free: free, Need: need, Enough: free >= need}
}

// moved carries labels along after a rename or move. A failure there is
// logged, the files are where they should be.
func (s *Service) moved(ctx context.Context, moves ...labels.Move) {
	if len(moves) == 0 {
		return
	}
	if err := labels.Moved(ctx, s.deps, moves...); err != nil {
		logger.Errorf("labels: %v", err)
	}
}

func (s *Service) copied(ctx context.Context, copies ...labels.Move) {
	if len(copies) == 0 {
		return
	}
	if err := labels.Copied(ctx, s.deps, copies...); err != nil {
		logger.Errorf("labels: %v", err)
	}
}

func (s *Service) removed(ctx context.Context, paths ...string) {
	if len(paths) == 0 {
		return
	}
	if err := labels.Removed(ctx, s.deps, paths...); err != nil {
		logger.Errorf("labels: %v", err)
	}
}
