package accounts_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pquerna/otp/totp"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/auth"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
	"github.com/junkerderprovinz/parleyport/kit/signin/signintest"
)

const totpSecret = "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"

// withSecondFactor gives u the TOTP secret and recovery code of the
// version before.
func (w *world) withSecondFactor(u *users.User, recovery string) {
	w.t.Helper()
	sealed, nonce, err := auth.SealTotpSecret(totpSecret)
	if err != nil {
		w.t.Fatal(err)
	}
	u.TOTPSecret, u.TOTPNonce, u.OtpEnabled = sealed, nonce, true
	if err = state.UpdateUser(u, "", "totpSecret", "totpNonce", "otpEnabled"); err != nil {
		w.t.Fatal(err)
	}
	hash, err := utils.HashPwd(recovery)
	if err != nil {
		w.t.Fatal(err)
	}
	if _, err := w.deps.DB.Exec(`INSERT INTO recovery_codes (user_id, code_hash) VALUES (?, ?)`, strconv.FormatUint(u.ID, 10), hash); err != nil {
		w.t.Fatal(err)
	}
}

// signInWithCode signs name in with the password and then code.
func (w *world) signInWithCode(name, code string) *browser {
	w.t.Helper()
	b := w.browser()
	step := b.do("POST", "/api/signin/v1/login", map[string]string{"name": name, "password": strong})
	if step.Code != http.StatusOK || !strings.Contains(step.Body.String(), `"code"`) {
		w.t.Fatalf("the password step: %d %s", step.Code, step.Body)
	}
	b.do("POST", "/api/signin/v1/login/totp", map[string]string{"code": code})
	return b
}

func TestTheImportCarriesPasswordSecondFactorRecoveryCodesAndRole(t *testing.T) {
	w := newWorld(t)
	anna := w.oldUser("anna", strong, false)
	w.withSecondFactor(&anna, "ABCD2345")
	seen := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	if _, err := w.deps.DB.Exec(`INSERT INTO account_profiles (user_id, seen_at) VALUES (?, ?)`, strconv.FormatUint(anna.ID, 10), seen.Unix()); err != nil {
		t.Fatal(err)
	}
	w.open()

	acc := w.account(anna)
	if acc.Name != "anna" || acc.Role != "user" || !acc.Password || !acc.TwoFactor || !acc.Seen.Equal(seen) {
		t.Fatalf("anna came over as %+v", acc)
	}
	if got := w.account(w.admin); got.Role != "admin" || got.Owner {
		t.Fatalf("jdp came over as %+v", got)
	}
	code, err := totp.GenerateCode(totpSecret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if b := w.signInWithCode("anna", code); b.session() == "" {
		t.Fatal("the old password and second factor do not sign in")
	}
	if b := w.signInWithCode("anna", "abcd-2345"); b.session() == "" {
		t.Fatal("the old recovery code does not sign in")
	}
	var used int64
	if err := w.deps.DB.QueryRow(`SELECT used_at FROM recovery_codes WHERE user_id = ?`, strconv.FormatUint(anna.ID, 10)).Scan(&used); err != nil || used == 0 {
		t.Fatalf("the code the new version took works once more in the old one: %d, %v", used, err)
	}
}

func TestAnUnreadableSecondFactorStaysASecondFactor(t *testing.T) {
	w := newWorld(t)
	settings.Config.Auth.TotpSecret = "the-first-key"
	if err := auth.InitializeEncryption(); err != nil {
		t.Fatal(err)
	}
	anna := w.oldUser("anna", strong, false)
	w.withSecondFactor(&anna, "ABCD2345")
	settings.Config.Auth.TotpSecret = "another-key"
	if err := auth.InitializeEncryption(); err != nil {
		t.Fatal(err)
	}
	w.open()

	if !w.account(anna).TwoFactor {
		t.Fatal("a secret that did not open came over as no second factor")
	}
	code, _ := totp.GenerateCode(totpSecret, time.Now())
	if b := w.signInWithCode("anna", code); b.session() != "" {
		t.Fatal("a code of the secret that did not open signs in")
	}
	if b := w.signInWithCode("anna", "ABCD2345"); b.session() == "" {
		t.Fatal("the recovery code does not get the account in")
	}
}

func TestImportedPasskeysSignInWithTheirOldHandleAndBackupFlags(t *testing.T) {
	w := newWorld(t).public()
	anna := w.oldUser("anna", strong, false)
	key := signintest.NewAuthenticator("https://" + host)
	key.Synced = true
	p, err := key.Passkey(host, []byte(strconv.FormatUint(anna.ID, 10)))
	if err != nil {
		t.Fatal(err)
	}
	b64 := base64.RawURLEncoding.EncodeToString
	anna.PasskeyCredentials = []users.WebAuthnCredential{{
		ID: b64(p.ID), PublicKey: b64(p.PublicKey), AAGUID: b64(p.AAGUID), SignCount: p.SignCount, Name: "Laptop", Origin: host,
		Flags: users.WebAuthnCredentialFlags{UserPresent: true, UserVerified: true, BackupEligible: p.BackupEligible, BackupState: p.BackupState},
	}}
	if err = state.UpdateUser(&anna, "", "PasskeyCredentials"); err != nil {
		t.Fatal(err)
	}
	w.open()

	b := w.browser()
	opts := b.do("POST", "/api/signin/v1/login/passkey/options", map[string]string{})
	if opts.Code != http.StatusOK {
		t.Fatalf("options: %d %s", opts.Code, opts.Body)
	}
	assertion, err := key.Get(opts.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if rec := b.do("POST", "/api/signin/v1/login/passkey", json.RawMessage(assertion)); rec.Code != http.StatusOK {
		t.Fatalf("the imported passkey does not sign in: %d %s", rec.Code, rec.Body)
	}
}

// oldToken registers an API token of the version before for u.
func (w *world) oldToken(u *users.User, name string, caps users.Permissions, minimal bool, readOnly bool) string {
	w.t.Helper()
	raw, claim, err := auth.MakeSignedTokenAPI(u, name, 30*24*time.Hour, caps, minimal)
	if err != nil {
		w.t.Fatal(err)
	}
	claim.Token, claim.Name, claim.ReadOnly = raw, name, readOnly
	if err := state.AddUserToken(u.Username, claim); err != nil {
		w.t.Fatal(err)
	}
	if err := state.AddApiToken(raw, u.ID); err != nil {
		w.t.Fatal(err)
	}
	return raw
}

func bearer(raw string) header { return header{"Authorization", "Bearer " + raw} }

func TestImportedTokensKeepWhatTheyWereAllowed(t *testing.T) {
	w := newWorld(t)
	full := w.oldToken(&w.admin, "voll", users.Permissions{}, true, false)
	reader := w.oldToken(&w.admin, "lesen", users.Permissions{}, true, true)
	capped := w.oldToken(&w.admin, "ohne-admin", users.Permissions{Api: true, Share: true}, false, false)
	revoked := w.oldToken(&w.admin, "weg", users.Permissions{}, true, false)
	if err := state.RevokeToken(revoked); err != nil {
		t.Fatal(err)
	}
	w.open()

	if me := decode[accounts.Me](t, w.must(http.StatusOK, w.req("GET", "/me", nil, bearer(full)))); me.Token == nil || me.Token.Rights != "rw" {
		t.Fatalf("a full token: %+v", me.Token)
	}
	w.must(http.StatusOK, w.req("GET", "/admin/users", nil, bearer(full)))
	if me := decode[accounts.Me](t, w.must(http.StatusOK, w.req("GET", "/me", nil, bearer(reader)))); me.Token.Rights != "r" {
		t.Fatalf("a read only token: %+v", me.Token)
	}
	w.must(http.StatusForbidden, w.req("GET", "/admin/users", nil, bearer(reader)))
	if rec := w.req("POST", "/admin/groups", map[string]string{"name": "neu"}, bearer(reader)); rec.Code != http.StatusForbidden || code(t, rec) != "tokenReadOnly" {
		t.Fatalf("a read only token wrote: %d %s", rec.Code, rec.Body)
	}
	w.must(http.StatusOK, w.req("GET", "/me", nil, bearer(capped)))
	w.must(http.StatusForbidden, w.req("GET", "/admin/users", nil, bearer(capped)))
	w.must(http.StatusUnauthorized, w.req("GET", "/me", nil, bearer(revoked)))
}

func TestTheOwnerIsTheAdminOfAServerThatWasOpen(t *testing.T) {
	w := newBareWorld(t)
	admin := w.oldUser(settings.PasswordAdminUsername(), "", true)
	w.oldUser("anna", "", false)
	w.open()
	if !w.auth.Anyone() || !state.OpenAccess() {
		t.Fatal("an open server came over closed")
	}
	if acc := w.account(admin); !acc.Owner {
		t.Fatalf("the admin of the open server is no owner: %+v", acc)
	}
	b := w.browser()
	w.must(http.StatusOK, b.do("PUT", "/api/signin/v1/me/password", map[string]string{"next": strong}))
	if !w.account(admin).Password || w.auth.Anyone() {
		t.Fatal("the first password did not go to the imported admin")
	}
}

func TestAClosedServerComesOverWithoutAnOwner(t *testing.T) {
	w := newWorld(t)
	admin := w.oldUser(settings.PasswordAdminUsername(), "", true)
	w.open()
	if w.account(admin).Owner || w.account(w.admin).Owner {
		t.Fatal("a closed server got an owner")
	}
}

func TestTwoNamesThatDifferInCaseStopTheStartWithBothNamed(t *testing.T) {
	w := newWorld(t)
	upper := w.oldUser("Anna", strong, false)
	w.oldUser("anna", strong, false)
	_, err := accounts.Open(context.Background(), w.deps)
	if err == nil || !strings.Contains(err.Error(), `"Anna"`) || !strings.Contains(err.Error(), `"anna"`) {
		t.Fatalf("the clash did not stop the start by name: %v", err)
	}

	t.Setenv("CC_SIGNIN_RENAME", strconv.FormatUint(upper.ID, 10)+":anna2")
	w.open()
	if u, err := state.GetUserByID(upper.ID); err != nil || u.Username != "anna2" {
		t.Fatalf("the rename did not take: %+v, %v", u.Username, err)
	}
	accounts.Close()
	w.open()
	if u, err := state.GetUserByUsername("anna2"); err != nil || u.ID != upper.ID {
		t.Fatalf("the variable left set changed the account again: %v", err)
	}
}

func TestAnOpenServersAdminNameMakesNoOwnerUnlessItIsAnAdmin(t *testing.T) {
	w := newBareWorld(t)
	named := w.oldUser(settings.PasswordAdminUsername(), "", false)
	w.oldUser("anna", "", true)
	w.open()
	if w.account(named).Owner {
		t.Fatal("an account that only carries the admin's name became the owner")
	}
}
