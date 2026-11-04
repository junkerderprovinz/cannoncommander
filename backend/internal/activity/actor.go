package activity

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

// Actor carries request-scoped identity for activity recording and filtering.
type Actor struct {
	User  *users.User
	Token string
}
