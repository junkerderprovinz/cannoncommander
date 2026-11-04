package access_test

import (
	"testing"
)

func TestRemoveUserFromAllGroups(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SetGroupMembers("editors", []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetGroupMembers("staff", []string{"alice"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveUserFromAllGroups("alice"); err != nil {
		t.Fatal(err)
	}
	if groups := s.GetUserGroups("alice"); len(groups) != 0 {
		t.Fatalf("alice should be in no groups, got %v", groups)
	}
	if got := s.GetGroupMembers()["editors"]; len(got) != 1 || got[0] != "bob" {
		t.Fatalf("expected [bob], got %v", got)
	}
	// The emptied group row still exists.
	if _, ok := s.GetGroupMembers()["staff"]; !ok {
		t.Fatal("empty group should remain")
	}
	sqlGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sqlGroups["staff"]["alice"]; ok {
		t.Fatal("alice should be removed from staff in SQL")
	}
	// Removing a user who is in no group is a no-op.
	if err := s.RemoveUserFromAllGroups("nobody"); err != nil {
		t.Fatal(err)
	}
}

func TestRenameUserInGroups(t *testing.T) {
	s, _, sqlStore := createTestStorageWithSQL(t)

	if err := s.SetGroupMembers("editors", []string{"alice", "bob"}); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameUserInGroups("alice", "alicia"); err != nil {
		t.Fatal(err)
	}
	got := s.GetGroupMembers()["editors"]
	if len(got) != 2 || got[0] != "alicia" || got[1] != "bob" {
		t.Fatalf("unexpected members: %v", got)
	}
	sqlGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := sqlGroups["editors"]["alicia"]; !ok {
		t.Fatal("alicia should be persisted in SQL")
	}
	if _, ok := sqlGroups["editors"]["alice"]; ok {
		t.Fatal("alice should be removed in SQL")
	}
	// Renaming a non-member or same name is a no-op.
	if err := s.RenameUserInGroups("nobody", "other"); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameUserInGroups("alicia", "alicia"); err != nil {
		t.Fatal(err)
	}
}
