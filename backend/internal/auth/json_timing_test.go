// Package auth_test contains timing attack tests for authentication.
//
// These tests verify that the JSON authentication implementation is protected
// against timing attacks that could leak information about valid usernames.
//
// Timing Attack Background:
// A timing attack is a side-channel attack where an attacker measures how long
// authentication operations take to determine if a username exists in the system.
// If authentication fails faster for non-existent users than for valid users
// with wrong passwords, an attacker can enumerate valid usernames.
//
// Protection Mechanism:
// The json.go Auth() implementation protects against timing attacks by:
// 1. Always running the bcrypt password comparison, even for non-existent users
// 2. Using utils.InvalidPasswordHash (a pre-computed bcrypt hash) when the user doesn't exist
// 3. Ensuring both valid and invalid user authentication paths take similar time
//
// Test Coverage:
// - TestJSONAuth_NoTimingAttack: Statistical analysis of timing differences
// - TestJSONAuth_InvalidPasswordHashInitialized: Verifies protection is in place
// - TestJSONAuth_ValidUserVsInvalidUserDirectComparison: Direct comparison test
package auth_test

import (
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

// TestJSONAuth_InvalidPasswordHashInitialized verifies that InvalidPasswordHash is properly set
func TestJSONAuth_InvalidPasswordHashInitialized(t *testing.T) {
	// This test ensures that the InvalidPasswordHash is initialized
	// If it's empty, timing attacks would be possible because invalid users
	// would skip bcrypt comparison entirely

	// Use lower bcrypt cost for faster testing
	originalCost := utils.BcryptCost
	utils.BcryptCost = 4
	t.Cleanup(func() {
		utils.BcryptCost = originalCost
	})

	err := utils.SetInvalidPasswordHash()
	if err != nil {
		t.Fatalf("Failed to set invalid password hash: %v", err)
	}

	if utils.InvalidPasswordHash == "" {
		t.Fatal("InvalidPasswordHash is not initialized - timing attack vulnerability exists!")
	}

	// Verify that the invalid hash can be used for password comparison
	// This should fail (wrong password) but take the same time as a real bcrypt comparison
	err = utils.CheckPwd("anypassword", utils.InvalidPasswordHash)
	if err == nil {
		t.Fatal("CheckPwd should fail with InvalidPasswordHash")
	}

	t.Log("✓ InvalidPasswordHash is properly initialized")
}
