package accounts_test

import (
	"context"
	"database/sql"
	"net/http"
	"slices"
	"sync"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
)

// ended keeps the accounts whose sessions and whose tokens the hooks heard
// ending, as a package with credentials of its own would.
var ended struct {
	sync.Mutex
	sessions, tokens []uint64
}

func init() {
	accounts.OnSessionsEnd(func(_ context.Context, _ *sql.DB, id uint64) error {
		ended.Lock()
		defer ended.Unlock()
		ended.sessions = append(ended.sessions, id)
		return nil
	})
	accounts.OnTokensEnd(func(_ context.Context, _ *sql.DB, id uint64) error {
		ended.Lock()
		defer ended.Unlock()
		ended.tokens = append(ended.tokens, id)
		return nil
	})
}

func heard(id uint64) (sessions, tokens bool) {
	ended.Lock()
	defer ended.Unlock()
	return slices.Contains(ended.sessions, id), slices.Contains(ended.tokens, id)
}

func TestChangingThePasswordEndsWhatCameInThroughASession(t *testing.T) {
	w := newWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.as("anna", "PUT", "/me/password", map[string]string{"current": strong, "next": "ein-ganz-neues-passwort"}))
	if sessions, tokens := heard(anna.ID); !sessions || tokens {
		t.Fatalf("sessions heard %v, tokens heard %v", sessions, tokens)
	}
}

func TestRevokingEverythingEndsTokensAndSessionsAlike(t *testing.T) {
	w := newWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"revoke": true}))
	if sessions, tokens := heard(anna.ID); !sessions || !tokens {
		t.Fatalf("sessions heard %v, tokens heard %v", sessions, tokens)
	}
}

func TestANewPasswordFromAnAdminEndsWhatCameInThroughASession(t *testing.T) {
	w := newWorld(t)
	anna := w.addUser("anna", "user", "/mnt/user")
	w.must(http.StatusOK, w.as("jdp", "PATCH", "/admin/users/anna", map[string]any{"password": "ein-ganz-neues-passwort"}))
	if sessions, _ := heard(anna.ID); !sessions {
		t.Fatal("the sessions ended without the hooks hearing of it")
	}
}
