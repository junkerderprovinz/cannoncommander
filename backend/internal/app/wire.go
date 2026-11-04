package app

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

// App holds wired dependencies for the running process.
type App struct {
	Store *state.Store
	Auth  *auth.Service
}

// WireServices connects domain packages to the state store after Open.
func WireServices(store *state.Store) (*App, error) {
	authSvc := auth.New(store)
	auth.SetDefault(authSvc)
	indexing.SetMetaStore(store)
	if err := auth.InitWebAuthn(store); err != nil {
		return nil, err
	}
	return &App{
		Store: store,
		Auth:  authSvc,
	}, nil
}

// MustWireServices is for tests; panics on wiring failure.
func MustWireServices(store *state.Store) *App {
	a, err := WireServices(store)
	if err != nil {
		panic(err)
	}
	return a
}
