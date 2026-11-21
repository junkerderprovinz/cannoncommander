package fileops

import (
	"context"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

// SweepParts runs the cleanup of part files.
func (s *Service) SweepParts(ctx context.Context) { s.sweepParts(ctx) }

// UseUnraid points the Unraid jobs at s until the returned func is called.
func UseUnraid(s *unraid.Service) (restore func()) {
	old := unraidService
	unraidService = func() *unraid.Service { return s }
	return func() { unraidService = old }
}

// TimesRefused reports whether a copy goes on when the target cannot take a file's times.
var TimesRefused = timesRefused
