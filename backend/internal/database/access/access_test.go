package access_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/access"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqldb"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/errors"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// sqlStoreAdapter adapts SQLStore to implement users.StorageBackend interface
type sqlStoreAdapter struct {
	store *sqldb.SQLStore
}

func (s *sqlStoreAdapter) GetBy(id uint64) (*users.User, error) {
	if id == 0 {
		return nil, errors.ErrNotExist
	}
	return s.store.GetUserByID(id)
}

func (s *sqlStoreAdapter) Gets() ([]*users.User, error) {
	return s.store.ListUsers()
}

func (s *sqlStoreAdapter) Save(user *users.User, changePass, disableScopeChange bool) error {
	// Check if user exists
	existingUser, err := s.store.GetUserByUsername(user.Username)
	if err != nil {
		// User doesn't exist - create new user
		if user.ID == 0 {
			nid, genErr := utils.RandomUint64ID()
			if genErr != nil {
				return genErr
			}
			user.ID = nid
		}
		return s.store.CreateUser(user)
	}
	// User exists - update
	user.ID = existingUser.ID
	return s.store.UpdateUser(user)
}

func (s *sqlStoreAdapter) Update(user *users.User, adminActor bool, fields ...string) error {
	return s.store.UpdateUser(user)
}

func (s *sqlStoreAdapter) DeleteByID(id uint64) error {
	return s.store.DeleteUserByID(id)
}

func createTestStorage(t *testing.T) (*access.Storage, *users.Storage) {
	t.Helper()
	accessStore, userStore, _ := createTestStorageWithSQL(t)
	return accessStore, userStore
}

func createTestStorageWithSQL(t *testing.T) (*access.Storage, *users.Storage, *sqldb.SQLStore) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "access_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	dbPath := filepath.Join(tempDir, "test.db")
	sqlStore, _, err := sqldb.NewSQLStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create SQL store: %v", err)
	}
	t.Cleanup(func() {
		sqlStore.Close()
	})

	adapter := &sqlStoreAdapter{store: sqlStore}
	userStore := users.NewStorage(adapter)
	accessStore := access.NewStorage(userStore)
	accessStore.SetSQLStore(sqlStore)
	return accessStore, userStore, sqlStore
}

func TestSyncUserGroups_WriteThroughAndReload(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SyncUserGroups("alice", []string{"acme", "staff"}); err != nil {
		t.Fatalf("SyncUserGroups: %v", err)
	}

	groups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatalf("GetAllGroups: %v", err)
	}
	if _, ok := groups["acme"]["alice"]; !ok {
		t.Fatalf("expected alice in acme in SQL, got %#v", groups)
	}
	if _, ok := groups["staff"]["alice"]; !ok {
		t.Fatalf("expected alice in staff in SQL, got %#v", groups)
	}

	// Simulate restart: new Storage loaded from SQL
	reloaded := access.NewStorage(nil)
	reloaded.Groups = groups
	if got := reloaded.GetUserGroups("alice"); len(got) != 2 {
		t.Fatalf("reloaded groups for alice: %v", got)
	}

	// Sync to a single group and ensure SQL drops the old membership
	if err = s.SyncUserGroups("alice", []string{"acme"}); err != nil {
		t.Fatalf("SyncUserGroups update: %v", err)
	}
	groups, err = sqlStore.GetAllGroups()
	if err != nil {
		t.Fatalf("GetAllGroups after update: %v", err)
	}
	if _, ok := groups["acme"]["alice"]; !ok {
		t.Fatal("alice should remain in acme")
	}
	if _, ok := groups["staff"]["alice"]; ok {
		t.Fatal("alice should no longer be in staff in SQL")
	}
}

func TestAddUserToGroup_WriteThrough(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.AddUserToGroup("editors", "bob"); err != nil {
		t.Fatalf("AddUserToGroup: %v", err)
	}
	groups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatalf("GetAllGroups: %v", err)
	}
	if _, ok := groups["editors"]["bob"]; !ok {
		t.Fatalf("expected bob in editors in SQL, got %#v", groups)
	}

	if err = s.RemoveUserFromGroup("editors", "bob"); err != nil {
		t.Fatalf("RemoveUserFromGroup: %v", err)
	}
	groups, err = sqlStore.GetAllGroups()
	if err != nil {
		t.Fatalf("GetAllGroups after remove: %v", err)
	}
	if _, ok := groups["editors"]; !ok {
		t.Fatal("expected editors group to remain after last member removed")
	}
	if _, still := groups["editors"]["bob"]; still {
		t.Fatal("bob should be removed from editors in SQL")
	}
}

func TestSetGroupMembers_ReplacesMembership(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SetGroupMembers("editors", []string{"alice", "bob", ""}); err != nil {
		t.Fatalf("SetGroupMembers: %v", err)
	}
	if err := s.SetGroupMembers("editors", []string{"bob", "carol"}); err != nil {
		t.Fatalf("SetGroupMembers replace: %v", err)
	}
	got := s.GetGroupMembers()["editors"]
	if len(got) != 2 || got[0] != "bob" || got[1] != "carol" {
		t.Fatalf("unexpected members: %v", got)
	}
	sqlGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatalf("GetAllGroups: %v", err)
	}
	if _, ok := sqlGroups["editors"]["alice"]; ok {
		t.Fatal("alice should have been removed in SQL")
	}
	if _, ok := sqlGroups["editors"]["carol"]; !ok {
		t.Fatal("carol should be persisted in SQL")
	}
}

func TestDeleteGroupRemovesItFromSQL(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SetGroupMembers("acme", []string{"alice"}); err != nil {
		t.Fatalf("SetGroupMembers: %v", err)
	}
	if err := s.DeleteGroup("acme"); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	sqlGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sqlGroups["acme"]; ok {
		t.Fatal("group row should be gone from SQL")
	}
	if got := s.GetUserGroups("alice"); len(got) != 0 {
		t.Fatalf("alice is still in %v", got)
	}
}
