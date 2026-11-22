package accounts

import (
	"context"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/parleyport/kit/signin"
)

// Ended is what CannonCommander does when the sign-in ended ways into an
// account, for tests that hand it an ending twice.
func Ended(d v1.Deps) func(ctx context.Context, account string, what signin.Ending) error {
	svc, err := serviceFor(d.DB)
	if err != nil {
		panic(err)
	}
	return handlers{db: d.DB, fs: d.FS, deps: d, svc: svc}.ended
}

// Drop is what CannonCommander does with the old fields of an imported
// account when the sign-in ended one of its ways in.
func Drop(d v1.Deps) func(ctx context.Context, account string, what signin.Ending) error {
	svc, err := serviceFor(d.DB)
	if err != nil {
		panic(err)
	}
	return handlers{db: d.DB, fs: d.FS, deps: d, svc: svc}.drop
}
