package web

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/app"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	_ "github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqldb" // Import to register SQL driver
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"golang.org/x/crypto/bcrypt"
)

func setupTestEnv(t *testing.T) {
	utils.BcryptCost = bcrypt.MinCost
	dbPath := filepath.Join(t.TempDir(), "test.sqlite")

	// Initialize state with SQLite database
	_, err := state.Initialize(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() {
		state.Close()
	})

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		"/srv": {
			Path: "/srv",
			Name: "srv",
		},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		"srv": {
			Path: "/srv",
			Name: "srv",
		},
	}
	// Initialize user resolvers so users package can resolve source names
	settings.InitializeUserResolvers()
}

func testSessionToken(t *testing.T, user *users.User, duration time.Duration) string {
	t.Helper()
	if duration == 0 {
		duration = time.Hour * 2
	}
	tokenString, _, err := auth.MakeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), duration, user.Permissions, true)
	if err != nil {
		t.Fatalf("MakeSignedTokenAPI: %v", err)
	}
	if err := state.RegisterSessionToken(tokenString, user.ID); err != nil {
		t.Fatalf("RegisterSessionToken: %v", err)
	}
	return tokenString
}

func TestWithAdminHelper(t *testing.T) {
	setupTestEnv(t)
	// Mock a user who has admin permissions
	adminUser := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Username:    "admin",
			Permissions: users.Permissions{Admin: true}, // Ensure the user is an admin
		},
	}
	nonAdminUser := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Username:    "non-admin",
			Permissions: users.Permissions{Admin: false}, // Non-admin user
		},
	}
	// Save the users to the mock database
	if err := state.CreateUser(adminUser, ""); err != nil {
		t.Fatal("failed to create admin user:", err)
	}
	adminUser.Permissions = users.Permissions{Admin: true}
	if err := state.UpdateUser(adminUser, "", "permissions"); err != nil {
		t.Fatal("failed to set admin permissions:", err)
	}
	if err := state.CreateUser(nonAdminUser, ""); err != nil {
		t.Fatal("failed to create non-admin user:", err)
	}
	nonAdminUser.Permissions = users.Permissions{Admin: false}
	if err := state.UpdateUser(nonAdminUser, "", "permissions"); err != nil {
		t.Fatal("failed to set non-admin permissions:", err)
	}
	// Test cases for different scenarios
	testCases := []struct {
		name               string
		expectedStatusCode int
		user               *users.User
	}{
		{
			name:               "Admin access allowed",
			expectedStatusCode: http.StatusOK, // Admin should be able to access
			user:               adminUser,
		},
		{
			name:               "Non-admin access forbidden",
			expectedStatusCode: http.StatusForbidden, // Non-admin should be forbidden
			user:               nonAdminUser,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tokenString := testSessionToken(t, tc.user, time.Hour*2)

			handler := withAdmin(mockHandler)

			recorder := httptest.NewRecorder()
			req, err := http.NewRequest(http.MethodGet, "/users", http.NoBody)
			if err != nil {
				t.Fatalf("Error creating request: %v", err)
			}
			req.AddCookie(&http.Cookie{
				Name:  "filebrowser_quantum_jwt",
				Value: tokenString,
			})

			handler(recorder, req)

			if recorder.Code != tc.expectedStatusCode {
				t.Errorf("\"%v\" expected status code %d, got %d", tc.name, tc.expectedStatusCode, recorder.Code)
			}
		})
	}
}

func mockHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	return http.StatusOK, nil // mock response
}

// withUser puts fn behind the login check that authenticateV1 runs.
func withUser(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withUserHelper(fn))
}

func withAdmin(fn handleFunc) http.HandlerFunc {
	return withUser(func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !d.User.Permissions.Admin {
			return http.StatusForbidden, nil
		}
		return fn(w, r, d)
	})
}
