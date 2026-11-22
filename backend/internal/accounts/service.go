package accounts

import (
	"database/sql"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// service holds what lives as long as the database: the rules.
type service struct {
	db    *sql.DB
	rules *ruleBook
}

// serviceFor returns the service for db and builds it on first use. Without
// a database, as in tests that only look at the contract, it is empty.
func serviceFor(db *sql.DB) (*service, error) {
	if db == nil {
		return &service{rules: &ruleBook{}}, nil
	}
	return v1.Shared(v1.Deps{DB: db}, "accounts", func() (*service, error) {
		book, err := loadRules(db, state.GetUserGroups)
		if err != nil {
			return nil, err
		}
		return &service{db: db, rules: book}, nil
	})
}

// Rules returns the access rules admins keep in the app. The server builds
// its path resolver with them.
func Rules(db *sql.DB) (ccfs.Rules, error) {
	if db == nil {
		return nil, nil
	}
	s, err := serviceFor(db)
	if err != nil {
		return nil, err
	}
	return s.rules, nil
}
