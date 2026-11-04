package state

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func TestResyncToolAccessDefaultsForAllUsers_appliesDefaults(t *testing.T) {
	initSidebarLinkTestDB(t)

	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "resync-user",
		},
	}
	if err := CreateUser(u, "password"); err != nil {
		t.Fatal(err)
	}

	doc := EffectiveToolAccessDefaults()
	for i := range doc.Items {
		if doc.Items[i].ToolID == users.ToolMaterialIconPicker {
			doc.Items[i].Enabled = false
			doc.Items[i].Enforced = false
			break
		}
	}
	toolAccessDefaultsMu.Lock()
	toolAccessDefaults = doc
	toolAccessDefaultsMu.Unlock()

	loaded, err := GetUserByUsername("resync-user")
	if err != nil {
		t.Fatal(err)
	}
	loaded.ToolAccess = users.ToolAccessMap{}
	if err = UpdateUser(&loaded, "", "toolAccess"); err != nil {
		t.Fatal(err)
	}

	if err = ResyncToolAccessDefaultsForAllUsers(); err != nil {
		t.Fatal(err)
	}

	loaded, err = GetUserByUsername("resync-user")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ToolAccess[users.ToolMaterialIconPicker] {
		t.Fatal("expected materialIconPicker denied after resync")
	}
}
