package unraid

import (
	"path/filepath"
	"sync"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

// stateTTL keeps a burst of requests from rereading the same files. emhttp
// rewrites them every few seconds at most.
const stateTTL = time.Second

// Service answers questions about the Unraid server. It is safe for
// concurrent use.
type Service struct {
	cfg Config

	mu     sync.Mutex
	cached *State
	readAt time.Time
	hub    *hub.Hub
	index  InodeIndex
	// onDevices hear when a pool or disk comes or goes.
	onDevices []func(devices []string)
}

// InodeIndex finds the other names of a hard link without walking the
// disk. ok is false when the index cannot answer for that device.
type InodeIndex interface {
	PathsByInode(device string, ino uint64) (rels []string, ok bool)
}

// New returns a service for the given paths.
func New(cfg Config) *Service { return &Service{cfg: cfg} }

var (
	defaultOnce sync.Once
	defaultSvc  *Service
)

// Default returns the service built from the CC_UNRAID_* variables. The
// job system and the listing use it for ParityBusy, Notify and DevicesOf.
func Default() *Service {
	defaultOnce.Do(func() { defaultSvc = New(ConfigFromEnv()) })
	return defaultSvc
}

// SetInodeIndex lets the file index answer hard link lookups.
func (s *Service) SetInodeIndex(ix InodeIndex) {
	s.mu.Lock()
	s.index = ix
	s.mu.Unlock()
}

// State returns the current server state, at most stateTTL old.
func (s *Service) State() (*State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Since(s.readAt) < stateTTL {
		return s.cached, nil
	}
	st, err := ReadState(s.cfg)
	if err != nil {
		return nil, err
	}
	s.cached, s.readAt = st, time.Now()
	return st, nil
}

// fresh drops the cache, so the next State reads the files again.
func (s *Service) fresh() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}

// ParityBusy reports whether a parity check, rebuild or clear is moving.
// Large copies wait while it does, because both share the same disks.
func (s *Service) ParityBusy() bool {
	st, err := s.State()
	return err == nil && st.Parity.Busy()
}

// UserShares is the folder that holds the user shares.
func (s *Service) UserShares() string { return filepath.Join(s.cfg.Mnt, "user") }

// ExclusivePool returns the pool that holds share when the share is exclusive
// and its link in the user shares leads to that pool.
func (s *Service) ExclusivePool(share string) (string, bool) {
	st, err := s.State()
	if err != nil {
		return "", false
	}
	pool, ok := st.exclusive[share]
	return pool, ok
}

// LinkedFolder reports whether name in the folder root is the link of an
// exclusive share, which stands for a folder of the user shares.
func (s *Service) LinkedFolder(root, name string) bool {
	if filepath.Clean(root) != s.UserShares() {
		return false
	}
	_, ok := s.ExclusivePool(name)
	return ok
}

// Asleep reports whether the named disk is spun down.
func (s *Service) Asleep(name string) bool {
	st, err := s.State()
	if err != nil {
		return false
	}
	d, ok := st.Disk(name)
	return ok && d.Spundown
}
