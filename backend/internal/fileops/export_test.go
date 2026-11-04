package fileops

import "github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"

// UseUnraid points the Unraid jobs at s until the returned func is called.
func UseUnraid(s *unraid.Service) (restore func()) {
	old := unraidService
	unraidService = func() *unraid.Service { return s }
	return func() { unraidService = old }
}
