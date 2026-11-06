package ccfs

import (
	"errors"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func TestAdminsReachASourceTheyWereNeverGivenAScopeFor(t *testing.T) {
	r, roots := newResolver(t, nil)
	shares := users.BackendScope{Path: roots.user, Scope: "/", Permissions: allPerms}
	jdp, anna := user("jdp", true, shares), user("anna", false, shares)

	if _, err := r.Check(jdp, "/mnt/disk1/filme", View|Create|Delete); err != nil {
		t.Errorf("admin on a source without a scope: %v", err)
	}
	if _, err := r.Check(jdp, "/boot/config", View); err != nil {
		t.Errorf("admin on the flash drive: %v", err)
	}
	if _, err := r.Check(anna, "/boot/config", View); !errors.Is(err, ErrForbidden) {
		t.Errorf("a plain user got a source nobody gave them: %v", err)
	}
	narrow := user("chef", true, users.BackendScope{Path: roots.user, Scope: "/filme", Permissions: allPerms})
	if _, err := r.Check(narrow, "/mnt/user/fotos", View); !errors.Is(err, ErrForbidden) {
		t.Errorf("an admin's own scope stopped counting: %v", err)
	}
	token := user("jdp", true, shares)
	token.RestrictToReading()
	if _, err := r.Check(token, "/boot/config", View); !errors.Is(err, ErrForbidden) {
		t.Errorf("a read only token of an admin reached a source without a scope: %v", err)
	}
}

func TestHomeIsTheFirstPlaceAUserMayOpen(t *testing.T) {
	r, roots := newResolver(t, nil)
	shares := users.BackendScope{Path: roots.user, Scope: "/", Permissions: allPerms}
	cases := []struct {
		name string
		u    *users.User
		want string
	}{
		{"admin", user("jdp", true, shares), "/mnt/user"},
		{"admin without a scope", user("root", true), "/mnt/user"},
		{"user with a folder of the shares", user("anna", false, users.BackendScope{Path: roots.user, Scope: "/fotos", Permissions: allPerms}), "/mnt/user/fotos"},
		{"user of the flash drive only", user("stick", false, users.BackendScope{Path: roots.boot, Scope: "/", Permissions: allPerms}), "/boot"},
		{"user without any place", user("gast", false), "/"},
	}
	for _, c := range cases {
		if got := r.Home(c.u); got != c.want {
			t.Errorf("%s: home = %q, want %q", c.name, got, c.want)
		}
	}
}
