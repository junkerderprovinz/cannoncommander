package auth

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ports"
)

// Service performs authentication using injected user reads. It never persists users.
type Service struct {
	users ports.UserReader
}

var defaultService *Service

// New constructs an auth service with the given user reader.
func New(users ports.UserReader) *Service {
	return &Service{users: users}
}

// SetDefault registers the process-wide auth service (called from app.WireServices).
func SetDefault(s *Service) {
	defaultService = s
}
