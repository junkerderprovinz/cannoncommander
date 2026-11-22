package access_test

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/access"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

type stubGroupSQL struct {
	saveErr   error
	deleteErr error
}

func (s *stubGroupSQL) SaveGroup(string, access.StringSet) error {
	return s.saveErr
}
func (s *stubGroupSQL) DeleteGroup(string) error { return s.deleteErr }
func (s *stubGroupSQL) SaveRevokedToken(string, int64) error {
	return nil
}
func (s *stubGroupSQL) PersistImmediateTokenRevocation(string) error         { return nil }
func (s *stubGroupSQL) PersistTokenRetirement(string, int64, []string) error { return nil }
func (s *stubGroupSQL) DeleteRevokedToken(string) error                      { return nil }
func (s *stubGroupSQL) SaveHashedToken(string, uint64, bool, int64) error    { return nil }
func (s *stubGroupSQL) DeleteHashedToken(string) error                       { return nil }
func (s *stubGroupSQL) DeleteHashedTokensByUserID(uint64) error {
	return nil
}

func TestAddUserToGroup_RollsBackOnSQLFailure(t *testing.T) {
	userStore := users.NewStorage(nil)
	store := access.NewStorage(userStore)
	store.SetSQLStore(&stubGroupSQL{saveErr: errors.New("save failed")})

	err := store.AddUserToGroup("editors", "alice")
	if err == nil {
		t.Fatal("expected SQL save error")
	}
	groups := store.GetUserGroups("alice")
	if len(groups) != 0 {
		t.Fatalf("expected in-memory rollback, alice groups = %#v", groups)
	}
}

func TestRemoveUserFromGroup_RollsBackOnSQLFailure(t *testing.T) {
	userStore := users.NewStorage(nil)
	store := access.NewStorage(userStore)
	if err := store.AddUserToGroup("editors", "alice"); err != nil {
		t.Fatalf("seed group: %v", err)
	}

	store.SetSQLStore(&stubGroupSQL{saveErr: errors.New("save failed")})
	err := store.RemoveUserFromGroup("editors", "alice")
	if err == nil {
		t.Fatal("expected SQL save error")
	}
	groups := store.GetUserGroups("alice")
	if len(groups) != 1 || groups[0] != "editors" {
		t.Fatalf("expected in-memory rollback, alice groups = %#v", groups)
	}
}
