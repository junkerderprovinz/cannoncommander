package activity

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ports"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// QueryDeps holds the access rules that filter activity queries.
type QueryDeps struct {
	Access ports.AccessGate
}

var queryDeps QueryDeps

// SetQueryDeps registers the access rules for activity queries (called from app.WireServices).
func SetQueryDeps(access ports.AccessGate) {
	queryDeps = QueryDeps{Access: access}
}

func accessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool {
	if queryDeps.Access == nil {
		return true
	}
	return queryDeps.Access.AccessPermitted(sourcePath, indexPath, username)
}
