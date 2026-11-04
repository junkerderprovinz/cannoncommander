package state

import (
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestInitSourceAccessDefaults_overlaysConfigOnRestart(t *testing.T) {
	t.Setenv("CC_ONLYOFFICE_SECRET", "")
	settings.Initialize("../../testdata/sourcedefaults.yaml")

	dbPath := filepath.Join(t.TempDir(), "filebrowser.sqlite")
	_, err := Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Close() })

	access := GetSourceAccessDefaults()
	if !access.Modify || !access.Create {
		t.Fatalf("initial source access defaults: %+v", access)
	}

	// Simulate admin changing modify in SQLite; config should re-overlay on next init.
	doc := sourceAccessSettingsDocument{
		DefaultPermissions: users.SourceFilePermissions{
			View: true, Download: true, Modify: false, Create: true, Delete: false,
		},
	}
	if saveErr := saveSourceAccessSettingsDocument(doc); saveErr != nil {
		t.Fatal(saveErr)
	}

	if err := InitSourceAccessDefaults(); err != nil {
		t.Fatal(err)
	}

	access = GetSourceAccessDefaults()
	if !access.Modify {
		t.Fatal("expected modify re-overlaid from the config on restart")
	}
	if settings.Env.ConfigSourceDefaultPermissions == nil {
		t.Fatal("expected config source default permissions from the yaml")
	}
}
