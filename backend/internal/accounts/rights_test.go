package accounts_test

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func TestATokenWithAnUnknownScopeIsRefused(t *testing.T) {
	u := users.User{FrontendUser: users.FrontendUser{Permissions: users.Permissions{Admin: true}}}
	if _, err := accounts.TokenRights(&u, []string{"all", "-admin", "files:write"}); err == nil {
		t.Fatal("a scope this server does not know passed")
	}
	if _, err := accounts.TokenRights(&u, []string{"all", "-admin"}); err != nil || u.Permissions.Admin {
		t.Fatalf("a capped token: admin %v, %v", u.Permissions.Admin, err)
	}
}
