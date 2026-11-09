package mcpapi

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

const (
	verifier    = "dBjftJeZ4CQP-0123456789abcdefghijklmnopqrstuvwxyz"
	claudeHook  = "https://claude.ai/api/mcp/auth_callback"
	testIssuer  = "https://files.example.com"
	oauthTarget = testIssuer + "/mcp"
)

func challengeOf(v string) string {
	sum := sha256.Sum256([]byte(v))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (e *env) post(path, contentType, body string) (int, map[string]any) {
	e.t.Helper()
	resp, err := http.Post(e.web.URL+path, contentType, strings.NewReader(body))
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(raw, &out)
	return resp.StatusCode, out
}

func (e *env) oauthOn() {
	e.t.Helper()
	if code := e.api("chef", http.MethodPut, "/mcp/oauth", map[string]any{"enabled": true, "issuer": testIssuer + "/"}, nil); code != http.StatusOK {
		e.t.Fatalf("switch OAuth on: %d", code)
	}
}

func (e *env) registerClient() string {
	e.t.Helper()
	code, out := e.post("/oauth/register", "application/json", `{"client_name":"Claude","redirect_uris":["`+claudeHook+`"],"token_endpoint_auth_method":"none"}`)
	if code != http.StatusCreated {
		e.t.Fatalf("register: %d %v", code, out)
	}
	return out["client_id"].(string)
}

// consent runs the consent page as user and returns the code the client gets.
func (e *env) consent(user, client string, canWrite bool) string {
	e.t.Helper()
	q := url.Values{"client_id": {client}, "redirect_uri": {claudeHook}, "response_type": {"code"},
		"code_challenge": {challengeOf(verifier)}, "code_challenge_method": {"S256"}, "state": {"xyz"}, "resource": {oauthTarget}}
	var info struct {
		Ticket  string
		Account string
		Client  struct{ Known string }
	}
	if code := e.api(user, http.MethodGet, "/mcp/oauth/authorize?"+q.Encode(), nil, &info); code != http.StatusOK || info.Account != user || info.Client.Known != "claude" {
		e.t.Fatalf("consent info: %d %+v", code, info)
	}
	var answer struct{ Redirect string }
	if code := e.api(user, http.MethodPost, "/mcp/oauth/authorize", map[string]any{"ticket": info.Ticket, "allow": true, "canWrite": canWrite}, &answer); code != http.StatusOK {
		e.t.Fatalf("consent: %d", code)
	}
	u, _ := url.Parse(answer.Redirect)
	if u.Query().Get("state") != "xyz" || u.Query().Get("iss") != testIssuer || !strings.HasPrefix(answer.Redirect, claudeHook) {
		e.t.Fatalf("redirect %s", answer.Redirect)
	}
	return u.Query().Get("code")
}

func (e *env) exchange(client, code string) (int, map[string]any) {
	return e.post("/oauth/token", "application/x-www-form-urlencoded", url.Values{
		"grant_type": {"authorization_code"}, "client_id": {client}, "code": {code},
		"redirect_uri": {claudeHook}, "code_verifier": {verifier}, "resource": {oauthTarget},
	}.Encode())
}

func TestOAuthIsOffUntilAnAdminSwitchesItOn(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/.well-known/oauth-authorization-server", "/.well-known/oauth-protected-resource/mcp"} {
		resp, _ := http.Get(e.web.URL + p)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s while off = %d", p, resp.StatusCode)
		}
	}
	if code, _ := e.post("/oauth/register", "application/json", `{"redirect_uris":["`+claudeHook+`"]}`); code != http.StatusNotFound {
		t.Fatalf("register while off = %d", code)
	}
	if code := e.api("anna", http.MethodPut, "/mcp/oauth", map[string]any{"enabled": true, "issuer": testIssuer}, nil); code != http.StatusForbidden {
		t.Fatalf("a user switching it on = %d", code)
	}
	var p problem
	if code := e.api("chef", http.MethodPut, "/mcp/oauth", map[string]any{"enabled": true, "issuer": "http://files.example.com"}, &p); code != http.StatusBadRequest || p.Code != "mcpOAuthIssuer" {
		t.Fatalf("a plain http issuer = %d %s", code, p.Code)
	}
	e.signIn = false
	if code := e.api("chef", http.MethodPut, "/mcp/oauth", map[string]any{"enabled": true, "issuer": testIssuer}, &p); code != http.StatusBadRequest || p.Code != "mcpOAuthNeedsSignIn" {
		t.Fatalf("without sign-in = %d %s", code, p.Code)
	}
	e.signIn = true
	e.oauthOn()
	resp, _ := http.Get(e.web.URL + "/.well-known/oauth-authorization-server")
	var meta map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&meta)
	resp.Body.Close()
	if meta["issuer"] != testIssuer || meta["token_endpoint"] != testIssuer+"/oauth/token" {
		t.Fatalf("metadata = %v", meta)
	}
	code, _, h := e.raw(http.MethodPost, initBody)
	if code != http.StatusUnauthorized || !strings.Contains(h.Get("WWW-Authenticate"), `resource_metadata="`+testIssuer+`/.well-known/oauth-protected-resource/mcp"`) {
		t.Fatalf("the challenge = %d %q", code, h.Get("WWW-Authenticate"))
	}
}

func TestAClientSignedInThroughOAuthWorksForTheUserWhoAllowedIt(t *testing.T) {
	e := newEnv(t)
	e.write("/mnt/user/filme/a.txt", "x")
	e.oauthOn()
	client := e.registerClient()
	code := e.consent("anna", client, false)
	status, tokens := e.exchange(client, code)
	if status != http.StatusOK || !strings.HasPrefix(tokens["access_token"].(string), accessPrefix) {
		t.Fatalf("token: %d %v", status, tokens)
	}
	s := e.client(tokens["access_token"].(string))
	out, errCode := e.tool(s, "get_access", nil)
	if errCode != "" || out["account"] != "anna" || out["viaOAuth"] != true || out["canWrite"] != false || out["key"] != "Claude" {
		t.Fatalf("get_access = %v %s", out, errCode)
	}
	if _, errCode := e.tool(s, "list_folder", map[string]any{"path": "/mnt/user/geheim"}); errCode != "not_found" {
		t.Fatalf("the grant reaches a hidden folder: %q", errCode)
	}
	if _, errCode := e.tool(s, "move_to_trash", map[string]any{"paths": []any{"/mnt/user/filme/a.txt"}}); errCode != "read_only_key" {
		t.Fatalf("a grant without writing trashed: %q", errCode)
	}
	if status, _ := e.exchange(client, code); status != http.StatusBadRequest {
		t.Fatalf("the code a second time = %d", status)
	}
	if status, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+tokens["access_token"].(string)); status != http.StatusUnauthorized {
		t.Fatalf("after the code came back, the grant still works: %d", status)
	}
}

func TestTheConsentIsBoundToTheSessionThatSawIt(t *testing.T) {
	e := newEnv(t)
	e.oauthOn()
	client := e.registerClient()
	q := url.Values{"client_id": {client}, "redirect_uri": {claudeHook}, "response_type": {"code"},
		"code_challenge": {challengeOf(verifier)}, "code_challenge_method": {"S256"}}
	var info struct{ Ticket string }
	e.api("anna", http.MethodGet, "/mcp/oauth/authorize?"+q.Encode(), nil, &info)
	var p problem
	if code := e.api("anna", http.MethodPost, "/mcp/oauth/authorize", map[string]any{"ticket": info.Ticket, "allow": true}, &p, "X-Session", "another session"); code != http.StatusConflict {
		t.Fatalf("the ticket from another session = %d %s", code, p.Code)
	}
	if code := e.api("anna", http.MethodPost, "/mcp/oauth/authorize", map[string]any{"ticket": info.Ticket, "allow": true}, nil, "X-Token", "1"); code != http.StatusForbidden {
		t.Fatalf("with an API token = %d", code)
	}
	bad := url.Values{}
	for k, v := range q {
		bad[k] = v
	}
	bad.Set("redirect_uri", "https://evil.example/cb")
	if code := e.api("anna", http.MethodGet, "/mcp/oauth/authorize?"+bad.Encode(), nil, &p); code != http.StatusBadRequest || p.Code != "oauthBadRedirect" {
		t.Fatalf("an unregistered redirect = %d %s", code, p.Code)
	}
	bad.Set("redirect_uri", claudeHook)
	bad.Set("code_challenge_method", "plain")
	if code := e.api("anna", http.MethodGet, "/mcp/oauth/authorize?"+bad.Encode(), nil, &p); code != http.StatusBadRequest {
		t.Fatalf("a plain challenge = %d", code)
	}
}

func TestARefreshTokenWorksOnceAndASecondUseEndsTheGrant(t *testing.T) {
	e := newEnv(t)
	e.oauthOn()
	client := e.registerClient()
	_, tokens := e.exchange(client, e.consent("anna", client, true))
	refresh := func(token string) (int, map[string]any) {
		return e.post("/oauth/token", "application/x-www-form-urlencoded", url.Values{
			"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {token}}.Encode())
	}
	status, next := refresh(tokens["refresh_token"].(string))
	if status != http.StatusOK || next["refresh_token"] == tokens["refresh_token"] {
		t.Fatalf("refresh: %d %v", status, next)
	}
	e.advance(time.Minute)
	if status, _ := refresh(tokens["refresh_token"].(string)); status != http.StatusBadRequest {
		t.Fatalf("the old refresh token = %d", status)
	}
	if status, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+next["access_token"].(string)); status != http.StatusUnauthorized {
		t.Fatalf("the grant after a reused refresh token = %d", status)
	}
}

func TestAnAccessTokenRunsOutAfterAnHour(t *testing.T) {
	e := newEnv(t)
	e.oauthOn()
	client := e.registerClient()
	_, tokens := e.exchange(client, e.consent("anna", client, false))
	access := tokens["access_token"].(string)
	if status, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+access); status != http.StatusOK {
		t.Fatalf("a fresh token = %d", status)
	}
	e.advance(accessTTL + time.Second)
	if status, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+access); status != http.StatusUnauthorized {
		t.Fatalf("an hour later = %d", status)
	}
}

func TestSwitchingOAuthOffEndsEveryGrant(t *testing.T) {
	e := newEnv(t)
	e.oauthOn()
	client := e.registerClient()
	_, tokens := e.exchange(client, e.consent("anna", client, false))
	e.key("anna", false)
	e.api("chef", http.MethodPut, "/mcp/oauth", map[string]any{"enabled": false}, nil)
	e.oauthOn()
	if status, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+tokens["access_token"].(string)); status != http.StatusUnauthorized {
		t.Fatalf("a grant from before = %d", status)
	}
	var list struct{ Keys, Revoked []KeyView }
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if len(list.Keys) != 1 || list.Keys[0].ViaOAuth || len(list.Revoked) != 1 || list.Revoked[0].RevokedReason != "oauth-off" {
		t.Fatalf("anna's keys = %+v", list)
	}
}

func TestTheClientRevokesItsOwnGrant(t *testing.T) {
	e := newEnv(t)
	e.oauthOn()
	client := e.registerClient()
	_, tokens := e.exchange(client, e.consent("anna", client, false))
	status, _ := e.post("/oauth/revoke", "application/x-www-form-urlencoded", url.Values{"client_id": {client}, "token": {tokens["refresh_token"].(string)}}.Encode())
	if status != http.StatusOK {
		t.Fatalf("revoke = %d", status)
	}
	if status, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+tokens["access_token"].(string)); status != http.StatusUnauthorized {
		t.Fatalf("after revoking = %d", status)
	}
}

func TestARegistrationIsCheckedAndThrottled(t *testing.T) {
	e := newEnv(t)
	e.oauthOn()
	for _, body := range []string{
		`{"redirect_uris":["http://evil.example/cb"]}`,
		`{"redirect_uris":["https://exаmple.com/cb"]}`,
		`{"redirect_uris":["https://example.com/cb#x"]}`,
		`{"redirect_uris":[]}`,
		`{"redirect_uris":["https://example.com/cb"],"grant_types":["client_credentials"]}`,
	} {
		if code, _ := e.post("/oauth/register", "application/json", body); code != http.StatusBadRequest {
			t.Fatalf("%s = %d", body, code)
		}
	}
	for i := 0; i < registrationsPerHour; i++ {
		e.post("/oauth/register", "application/json", `{"redirect_uris":["http://127.0.0.1:8000/cb"]}`)
	}
	if code, _ := e.post("/oauth/register", "application/json", `{"redirect_uris":["http://127.0.0.1:8000/cb"]}`); code != http.StatusTooManyRequests {
		t.Fatalf("one registration too many = %d", code)
	}
}
