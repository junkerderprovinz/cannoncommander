// Package ports defines narrow interfaces for dependency injection.
// Domain packages depend on these interfaces; state.Store implements them.
package ports

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/dbindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// UserReader loads persisted users (read-only).
type UserReader interface {
	GetUserByID(id uint64) (users.User, error)
	GetUserByUsername(username string) (users.User, error)
}

// UserWriter persists user changes. Handlers orchestrate writes; auth must not implement this.
type UserWriter interface {
	CreateUser(user *users.User, plaintextPassword string) error
	UpdateUser(user *users.User, plaintextPassword string, fields ...string) error
	DeleteUser(id uint64) error
}

// IndexMetaStore persists per-source index metadata (complexity, scanner state).
type IndexMetaStore interface {
	GetIndexInfo(path string) (dbindex.IndexInfo, error)
	SaveIndexInfo(info *dbindex.IndexInfo) error
	ResetAllIndexComplexities() error
}
