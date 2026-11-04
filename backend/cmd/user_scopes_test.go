package cmd

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestUpdateUserScopes_mergesMissingDefaultEnabled(t *testing.T) {
	settings.Initialize(settingsMigrationConfigPath(t))
	alignSettingsSourcesForMigrationFixture(t)

	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "graham"},
		BackendScopes: []users.BackendScope{
			{Path: fixturePlaywrightSource, Scope: "/myfolder"},
			{Path: fixtureDockerSource, Scope: "/"},
		},
	}

	if !updateUserScopes(user) {
		t.Fatal("expected missing defaultEnabled access source to be merged")
	}
	if len(user.BackendScopes) != 3 {
		t.Fatalf("scopes=%#v want 3 including access", user.BackendScopes)
	}
	found := false
	for _, scope := range user.BackendScopes {
		if scope.Path == fixtureAccessSource {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("access source should be added for defaultEnabled: %#v", user.BackendScopes)
	}
	// Preserve existing scope paths
	for _, scope := range user.BackendScopes {
		if scope.Path == fixturePlaywrightSource && scope.Scope != "/myfolder" {
			t.Fatalf("playwright scope path changed: %#v", scope)
		}
	}
}

func TestUpdateUserScopes_seedsDefaultEnabledWhenEmpty(t *testing.T) {
	settings.Initialize(settingsMigrationConfigPath(t))
	alignSettingsSourcesForMigrationFixture(t)

	user := &users.User{
		FrontendUser: users.FrontendUser{Username: "newuser"},
	}

	if !updateUserScopes(user) {
		t.Fatal("expected default-enabled scopes to be seeded")
	}
	if len(user.BackendScopes) != 3 {
		t.Fatalf("scopes=%#v want all default-enabled sources", user.BackendScopes)
	}
}
