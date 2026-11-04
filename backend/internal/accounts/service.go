package accounts

import (
	"database/sql"
	"time"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
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
	}
}

// Gate adds the rules admins keep in the app to the access gate of the
// login system, so the file system follows both. The server builds its
// path resolver with it.
func Gate(inner ccfs.AccessGate, db *sql.DB) (ccfs.AccessGate, error) {
	if db == nil {
		return inner, nil
	}
	s, err := serviceFor(db)
	if err != nil {
		return nil, err
	}
	return gate{inner: inner, rules: s.rules}, nil
}

// RecoveryCodes lets the password login take a recovery code instead of a
// TOTP code.
func RecoveryCodes(db *sql.DB) (auth.RecoveryCodes, error) {
	s, err := serviceFor(db)
	if err != nil {
		return nil, err
	}
	return s.recovery, nil
}
