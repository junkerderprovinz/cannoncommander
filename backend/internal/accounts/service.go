package accounts

import (
	"database/sql"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
)

// service holds what lives as long as the database: the rules, pending two
// factor setups, sign ins halfway through, reset codes and the counters of
// failed attempts.
type service struct {
	db       *sql.DB
	rules    *ruleBook
	recovery *recovery
	totp     *totpSetups
	passkeys *passkeyStarts
	resets   *resets
	signIns  *pendingSignIns

	passwordTries *limiter
	factorTries   *limiter
	resetTries    *limiter
	signInTries   *limiter
	addressTries  *limiter

	basicTries        *limiter
	basicAddressTries *limiter
	basicSeen         *passwordMemory
}

// serviceFor returns the service for db and builds it on first use. Without
// a database, as in tests that only look at the contract, it is empty.
func serviceFor(db *sql.DB) (*service, error) {
	if db == nil {
		return newService(nil, nil), nil
	}
	return v1.Shared(v1.Deps{DB: db}, "accounts", func() (*service, error) {
		book, err := loadRules(db, state.GetUserGroups)
		if err != nil {
			return nil, err
		}
		return newService(db, book), nil
	})
}

func newService(db *sql.DB, book *ruleBook) *service {
	if book == nil {
		book = &ruleBook{}
	}
	return &service{
		db:            db,
		rules:         book,
		recovery:      &recovery{db: db},
		totp:          newTotpSetups(),
		passkeys:      newPasskeyStarts(),
		resets:        newResets(),
		signIns:       newPendingSignIns(),
		passwordTries: newLimiter(5, 15*time.Minute),
		factorTries:   newLimiter(5, 15*time.Minute),
		resetTries:    newLimiter(5, 15*time.Minute),
		signInTries:   newLimiter(5, 15*time.Minute),
		addressTries:  newLimiter(20, 15*time.Minute),

		basicTries:        newLimiter(5, 15*time.Minute),
		basicAddressTries: newLimiter(20, 15*time.Minute),
		basicSeen:         newPasswordMemory(),
	}
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
