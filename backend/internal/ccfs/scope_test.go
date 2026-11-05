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
