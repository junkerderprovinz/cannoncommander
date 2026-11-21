package accounts

import (
	"context"
	"testing"

	"github.com/junkerderprovinz/parleyport/kit/signin"
)

func TestARememberedPasswordEndsWithTheAccountsEpochBlockOrSecondFactor(t *testing.T) {
	m := newPasswordMemory()
	now := signin.Account{ID: "7", Password: true, Epoch: 3}
	lookup := func(context.Context, string) (signin.Account, error) { return now, nil }
	m.remember("anna", "geheim", now)
	if _, ok := m.knows(t.Context(), lookup, "anna", "geheim"); !ok {
		t.Fatal("a password just checked is not remembered")
	}
	if _, ok := m.knows(t.Context(), lookup, "anna", "anders"); ok {
		t.Fatal("another password counts as remembered")
	}
	for name, change := range map[string]func(*signin.Account){
		"a new epoch": func(a *signin.Account) { a.Epoch++ },
		"a block":     func(a *signin.Account) { a.Blocked = true },
		"two-factor":  func(a *signin.Account) { a.TwoFactor = true },
		"no password": func(a *signin.Account) { a.Password = false },
	} {
		now = signin.Account{ID: "7", Password: true, Epoch: 3}
		change(&now)
		if _, ok := m.knows(t.Context(), lookup, "anna", "geheim"); ok {
			t.Errorf("the memory outlives %s", name)
		}
	}
}
