package system

import (
	"context"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

// ConnectOffice tries the office server o and, when it answers, saves it
// as the one office files open in. It returns what the test found and the
// office settings without the secret.
func ConnectOffice(ctx context.Context, d v1.Deps, o Office) (IntegrationResult, Office, error) {
	s := &service{store: storeFor(d), apply: apply, test: newTester().run}
	return s.connectOffice(ctx, o)
}

// connectOffice keeps the saved secret when o comes without one for the
// saved address, as the settings page does.
func (s *service) connectOffice(ctx context.Context, o Office) (IntegrationResult, Office, error) {
	o.URL = strings.TrimRight(strings.TrimSpace(o.URL), "/")
	saved, err := s.store.get(ctx)
	if err != nil {
		return IntegrationResult{}, Office{}, err
	}
	if o.Secret == "" && o.URL == saved.Office.URL {
		o.Secret = saved.Office.Secret
	}
	res := s.test(ctx, o.Kind, o.URL, o.Secret)
	if !res.Reachable {
		return res, saved.public().Office, nil
	}
	a, err := s.store.update(ctx, func(a *App) error {
		a.Office = o
		return nil
	})
	if err != nil {
		return res, Office{}, err
	}
	s.apply(a)
	return res, a.public().Office, nil
}
