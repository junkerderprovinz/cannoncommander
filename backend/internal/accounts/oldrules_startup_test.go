package accounts_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// TestOldRulesAreCarriedOverAtStart opens a database that still holds the
// access rules of the older interface, as a server does on its first start
// with this version, and checks what each user gets to see afterwards.
func TestOldRulesAreCarriedOverAtStart(t *testing.T) {
	base := t.TempDir()
	saved := settings.Config
	t.Cleanup(func() { settings.Config = saved })

	share, access := filepath.Join(base, "user"), filepath.Join(base, "access")
	for _, d := range []string{"user/text-files", "user/Filme/neu", "user/fotos/privat", "access/inbox"} {
		if err := os.MkdirAll(filepath.Join(base, filepath.FromSlash(d)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(share, "text-files", "bash.sh"), []byte("#!/bin/sh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sources := []*settings.Source{{Name: "Shares", Path: share}, {Name: "access", Path: access}}
	sources[1].Config.DenyByDefault = true
	settings.Config.Server.Sources = sources
	settings.Config.Server.SourceMap = map[string]*settings.Source{share: sources[0], access: sources[1]}
	settings.Config.Server.NameToSource = map[string]*settings.Source{"Shares": sources[0], "access": sources[1]}
	settings.InitializeUserResolvers()

	dbPath := filepath.Join(base, "state.db")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	all := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: true, Delete: true, Configured: true}
	for _, name := range []string{"admin", "basic", "bert", "cleo"} {
		u := users.User{FrontendUser: users.FrontendUser{Username: name, LoginMethod: users.LoginMethodPassword}}
		if err := state.CreateUser(&u, strong); err != nil {
			t.Fatal(err)
		}
		u.BackendScopes = []users.BackendScope{{Path: share, Scope: "/", Permissions: all}, {Path: access, Scope: "/", Permissions: all}}
		if err := state.UpdateUser(&u, "", "backendScopes"); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.SetGroupMembers("team", []string{"bert"}); err != nil {
		t.Fatal(err)
	}
	if err := state.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE access_rules (id INTEGER PRIMARY KEY AUTOINCREMENT, source TEXT NOT NULL, path TEXT NOT NULL, rule_data TEXT NOT NULL, UNIQUE(source, path))`,
		`INSERT INTO access_rules (source, path, rule_data) VALUES
			('` + share + `', '/text-files/bash.sh/', '{"Deny":{"Users":{"admin":{}},"Groups":{}},"Allow":{"Users":{},"Groups":{}}}'),
			('` + share + `', '/filme/', '{"denyAll":true,"Deny":{"Users":{},"Groups":{}},"Allow":{"Users":{},"Groups":{"team":{}}}}'),
			('` + share + `', '/fotos/privat/', '{"Deny":{"Users":{"cleo":{}},"Groups":{}},"Allow":{"Users":{},"Groups":{}}}'),
			('` + access + `', '/', '{"Deny":{"Users":{"basic":{}},"Groups":{}},"Allow":{"Users":{"cleo":{}},"Groups":{}}}')`,
		`INSERT INTO settings (key, value) VALUES ('access_rules_lowercase_migrated', 'true')`,
		`DELETE FROM cc_migrations WHERE name = 'accounts/007_old_access_rules'`,
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { state.Close() })
	var left int
	if err := state.MainDB().QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'access_rules'`).Scan(&left); err != nil || left != 0 {
		t.Fatalf("the old table is still there (%d, %v)", left, err)
	}
	rules, err := accounts.Rules(state.MainDB())
	if err != nil {
		t.Fatal(err)
	}
	fs, err := ccfs.New(rules,
		ccfs.Source{Name: "Shares", Mount: ccfs.MountFor(share), Root: share},
		ccfs.Source{Name: "access", Mount: ccfs.MountFor(access), Root: access},
	)
	if err != nil {
		t.Fatal(err)
	}
	at := func(root, p string) string { return ccfs.MountFor(root) + p }
	cases := []struct {
		user, path string
		visible    bool
	}{
		{"admin", at(share, "/text-files/bash.sh"), false},
		{"basic", at(share, "/text-files/bash.sh"), true},
		{"admin", at(share, "/Filme/neu"), false},
		{"bert", at(share, "/Filme/neu"), true},
		{"cleo", at(share, "/fotos/privat"), false},
		{"cleo", at(share, "/fotos"), true},
		{"basic", at(access, "/inbox"), false},
		{"cleo", at(access, "/inbox"), true},
		{"admin", at(access, "/inbox"), false},
	}
	for _, c := range cases {
		u, err := state.GetUserByUsername(c.user)
		if err != nil {
			t.Fatal(err)
		}
		_, err = fs.Check(&u, c.path, ccfs.View)
		if c.visible && err != nil || !c.visible && !errors.Is(err, ccfs.ErrForbidden) {
			t.Errorf("%s at %s: err = %v, want visible=%v", c.user, c.path, err, c.visible)
		}
	}
}
