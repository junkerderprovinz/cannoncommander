package mcpapi

import (
	"net/http"
	"net/url"
	"testing"
)

func TestAKeyMadeWhileOpenEndsWhenTheServerCloses(t *testing.T) {
	e := newEnv(t)
	closedKey, _ := e.key("anna", false)
	e.signIn = false
	openKey, _ := e.key("anna", false)
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+openKey); code != http.StatusOK {
		t.Fatalf("the key made while open does not work while open: %d", code)
	}
	e.signIn = true
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+openKey); code != http.StatusUnauthorized {
		t.Fatalf("the key made while open works after the close: %d", code)
	}
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+closedKey); code != http.StatusOK {
		t.Fatalf("a key made with a sign-in stopped working: %d", code)
	}
	var list struct{ Revoked []KeyView }
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if len(list.Revoked) != 1 || list.Revoked[0].RevokedReason != "closed" {
		t.Fatalf("anna sees %+v", list.Revoked)
	}
	e.signIn = false
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+openKey); code != http.StatusUnauthorized {
		t.Fatalf("opening again brings the key back: %d", code)
	}
}

func TestAKeyReplacedWhileOpenEndsWhenTheServerCloses(t *testing.T) {
	e := newEnv(t)
	e.key("chef", false)
	_, id := e.key("anna", false)
	e.signIn = false
	var out struct{ Key string }
	if code := e.api("anna", http.MethodPost, "/mcp/keys/"+id+"/rotate", nil, &out); code != http.StatusOK {
		t.Fatalf("replace while open: %d", code)
	}
	e.signIn = true
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+out.Key); code != http.StatusUnauthorized {
		t.Fatalf("the secret handed out while open works after the close: %d", code)
	}
}

func TestAClientAllowedWhileOpenEndsWhenTheServerCloses(t *testing.T) {
	e := newEnv(t)
	e.signIn = false
	e.oauthOn()
	client := e.registerClient()
	q := url.Values{"client_id": {client}, "redirect_uri": {claudeHook}, "response_type": {"code"},
		"code_challenge": {challengeOf(verifier)}, "code_challenge_method": {"S256"}, "state": {"xyz"}, "resource": {oauthTarget}}
	var info struct{ Ticket string }
	e.api("anna", http.MethodGet, "/mcp/oauth/authorize?"+q.Encode(), nil, &info, "X-Session", "")
	var answer struct{ Redirect string }
	e.api("anna", http.MethodPost, "/mcp/oauth/authorize", map[string]any{"ticket": info.Ticket, "allow": true}, &answer, "X-Session", "")
	u, _ := url.Parse(answer.Redirect)
	status, tokens := e.exchange(client, u.Query().Get("code"))
	if status != http.StatusOK {
		t.Fatalf("exchange while open: %d %v", status, tokens)
	}
	access, _ := tokens["access_token"].(string)
	e.signIn = true
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+access); code != http.StatusUnauthorized {
		t.Fatalf("the access token allowed while open works after the close: %d", code)
	}
	status, body := e.post("/oauth/token", "application/x-www-form-urlencoded", url.Values{
		"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {tokens["refresh_token"].(string)}}.Encode())
	if status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("the refresh token allowed while open works after the close: %d %v", status, body)
	}
}

func TestAKeyMadeInALaterOpenSpellIsRevokedWhenTheServerClosesAgain(t *testing.T) {
	e := newEnv(t)
	closedKey, _ := e.key("chef", false)
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+closedKey); code != http.StatusOK {
		t.Fatalf("a key made with a sign-in: %d", code)
	}
	e.signIn = false
	e.key("anna", false)
	e.signIn = true
	var list struct{ Revoked []KeyView }
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if len(list.Revoked) != 1 || list.Revoked[0].RevokedReason != "closed" {
		t.Fatalf("anna sees %+v", list.Revoked)
	}
}
