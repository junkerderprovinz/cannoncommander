package files

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ports"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing/iteminfo"
)

// ChildAccessChecker filters directory listings by access rules.
type ChildAccessChecker interface {
	CheckChildItemAccess(response *iteminfo.FileInfo, idx *indexing.Index, username string) error
}

// Service performs filesystem operations with injected access dependencies.
type Service struct {
	access ports.AccessGate
	child  ChildAccessChecker
}

var defaultService *Service

// New constructs a file service with the given port implementations.
func New(access ports.AccessGate, child ChildAccessChecker) *Service {
	return &Service{access: access, child: child}
}

// SetDefault registers the process-wide file service (called from app.WireServices).
func SetDefault(s *Service) {
	defaultService = s
}

func svc() *Service {
	if defaultService != nil {
		return defaultService
	}
	return &Service{}
}

func (s *Service) accessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool {
	if s.access == nil {
		return true
	}
	return s.access.AccessPermitted(sourcePath, indexPath, username)
}

func (s *Service) checkChildItemAccess(response *iteminfo.FileInfo, idx *indexing.Index, username string) error {
	if s.child == nil {
		return nil
	}
	return s.child.CheckChildItemAccess(response, idx, username)
}

// CheckPermissions validates user access and returns the resolved index path and user scope.
func (s *Service) CheckPermissions(opts utils.FileOptions, user *users.User) (string, string, error) {
	return checkPermissionsImpl(opts, user, s)
}

// FileInfoFaster returns extended file info with access filtering applied.
func (s *Service) FileInfoFaster(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
	return fileInfoFasterImpl(opts, user, s)
}
