package quota

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// CopyMoveContext describes a copy/move/rename admission check.
type CopyMoveContext struct {
	Principal       *users.User
	Action          string
	FromSourceName  string
	ToSourceName    string
	FromSourcePath  string
	ToSourcePath    string
	FromIndexPath   string
	ToIndexPath     string
	IsSrcDir        bool
	OverwriteBytes  int64
	ShareHash       string
	ShareLimitBytes int64
	SessionID       string
}
