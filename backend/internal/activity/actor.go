package activity

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/share"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// Actor carries request-scoped identity for activity recording and filtering.
type Actor struct {
	User  *users.User
	Share share.Share
	Token string
}
