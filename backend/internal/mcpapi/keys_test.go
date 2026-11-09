package mcpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

type problem struct {
	Code string `json:"code"`
}

func TestAKeyIsShownOnceAndStoredAsAFingerprint(t *testing.T) {
	e := newEnv(t)
	key, id := e.key("anna", false)
	if !strings.HasPrefix(key, keyPrefix) || len(key) != len(keyPrefix)+encodedSecretLength {
		t.Fatalf("key %q", key)
	}
	row, err := e.srv.st.key(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if row.Digest == "" || strings.Contains(row.Digest, key[len(keyPrefix):]) || row.Hint != key[len(key)-4:] {
		t.Fatalf("stored %+v", row)
	}
	var list map[string]any
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if raw := mustJSON(t, list); strings.Contains(raw, key) || strings.Contains(raw, row.Digest) {
		t.Fatalf("the list gives the key away: %s", raw)
	}
}

func TestANewKeyOnlyReadsUnlessAskedOtherwise(t *testing.T) {
	e := newEnv(t)
	var out struct{ Item KeyView }
	e.api("anna", http.MethodPost, "/mcp/keys", map[string]any{"label": "Claude", "password": "richtig"}, &out)
	if out.Item.CanWrite {
		t.Fatal("a key made without canWrite may write")
	}
}

func TestMakingAKeyNeedsTheOwnerAtTheKeyboard(t *testing.T) {
	e := newEnv(t)
	body := map[string]any{"label": "Claude"}
	var p problem
	if code := e.api("anna", http.MethodPost, "/mcp/keys", body, &p); code != http.StatusForbidden || p.Code != "currentPasswordWrong" {
		t.Fatalf("without the password: %d %s", code, p.Code)
	}
	body["password"] = "richtig"
	if code := e.api("anna", http.MethodPost, "/mcp/keys", body, &p, "X-Token", "1"); code != http.StatusForbidden || p.Code != "tokenCannotChange" {
		t.Fatalf("with an API token: %d %s", code, p.Code)
	}
	if code := e.api("anna", http.MethodPost, "/mcp/keys", body, &p, "X-Session", ""); code != http.StatusForbidden {
		t.Fatalf("without a session: %d", code)
	}
	if code := e.api("anna", http.MethodPost, "/mcp/keys", body, nil); code != http.StatusCreated {
		t.Fatalf("with session and password: %d", code)
	}
}

func TestWithoutSignInAKeyIsOnlyMadeFromALocalAddress(t *testing.T) {
	e := newEnv(t)
	e.signIn = false
	body := map[string]any{"label": "Claude"}
	var p problem
	if code := e.api("anna", http.MethodPost, "/mcp/keys", body, &p, "Host", "files.example.com"); code != http.StatusForbidden || p.Code != "mcpKeyNeedsSignIn" {
		t.Fatalf("from a public name: %d %s", code, p.Code)
	}
	for _, host := range []string{"127.0.0.1:8080", "tower", "nas.lan", "[::1]:80"} {
		body["label"] = "Claude " + host
		if code := e.api("anna", http.MethodPost, "/mcp/keys", body, &p, "Host", host); code != http.StatusCreated {
			t.Fatalf("from %s: %d %s", host, code, p.Code)
		}
	}
}

func TestAnAccountHasTenKeysAndItsOwnNames(t *testing.T) {
	e := newEnv(t)
	for i := 0; i < keyLimit; i++ {
		e.key("anna", false)
	}
	var p problem
	if code := e.api("anna", http.MethodPost, "/mcp/keys", map[string]any{"label": "one more", "password": "richtig"}, &p); code != http.StatusConflict || p.Code != "mcpKeyLimit" {
		t.Fatalf("key %d: %d %s", keyLimit+1, code, p.Code)
	}
	if code := e.api("gast", http.MethodPost, "/mcp/keys", map[string]any{"label": "one more", "password": "richtig"}, nil); code != http.StatusCreated {
		t.Fatalf("another account: %d", code)
	}
	if code := e.api("gast", http.MethodPost, "/mcp/keys", map[string]any{"label": "ONE MORE", "password": "richtig"}, &p); code != http.StatusConflict || p.Code != "mcpKeyLabelTaken" {
		t.Fatalf("a name taken in another case: %d %s", code, p.Code)
	}
}

func TestOnlyTheOwnerSwitchesWritingOn(t *testing.T) {
	e := newEnv(t)
	_, id := e.key("anna", false)
	var p problem
	if code := e.api("anna", http.MethodPatch, "/mcp/keys/"+id, map[string]any{"canWrite": true}, &p); code != http.StatusForbidden {
		t.Fatalf("on without the password: %d %s", code, p.Code)
	}
	if code := e.api("chef", http.MethodPatch, "/mcp/keys/"+id, map[string]any{"canWrite": true, "password": "richtig"}, &p); code != http.StatusForbidden || p.Code != "mcpKeyNotYours" {
		t.Fatalf("an admin switching on: %d %s", code, p.Code)
	}
	if code := e.api("gast", http.MethodPatch, "/mcp/keys/"+id, map[string]any{"canWrite": false}, &p); code != http.StatusNotFound {
		t.Fatalf("somebody else: %d %s", code, p.Code)
	}
	var k KeyView
	if code := e.api("anna", http.MethodPatch, "/mcp/keys/"+id, map[string]any{"canWrite": true, "password": "richtig"}, &k); code != http.StatusOK || !k.CanWrite {
		t.Fatalf("the owner with the password: %d %+v", code, k)
	}
	if code := e.api("chef", http.MethodPatch, "/mcp/keys/"+id, map[string]any{"canWrite": false}, &k); code != http.StatusOK || k.CanWrite {
		t.Fatalf("an admin switching off: %d %+v", code, k)
	}
}

func TestAReplacedKeyStopsWorkingAtOnce(t *testing.T) {
	e := newEnv(t)
	old, id := e.key("anna", false)
	var out struct {
		Key  string
		Item KeyView
	}
	if code := e.api("chef", http.MethodPost, "/mcp/keys/"+id+"/rotate", map[string]any{"password": "richtig"}, nil); code != http.StatusForbidden {
		t.Fatalf("an admin replacing anna's key: %d", code)
	}
	if code := e.api("anna", http.MethodPost, "/mcp/keys/"+id+"/rotate", map[string]any{"password": "richtig"}, &out); code != http.StatusOK || out.Item.ID != id {
		t.Fatalf("replace: %d %+v", code, out)
	}
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+old); code != http.StatusUnauthorized {
		t.Fatalf("the old key = %d", code)
	}
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+out.Key); code != http.StatusOK {
		t.Fatalf("the new key = %d", code)
	}
}

func TestAnAdminSeesAndRevokesEverybodysKeys(t *testing.T) {
	e := newEnv(t)
	key, id := e.key("anna", true)
	e.key("chef", false)
	var p problem
	if code := e.api("anna", http.MethodGet, "/mcp/keys?all=true", nil, &p); code != http.StatusForbidden {
		t.Fatalf("a user asking for all keys: %d", code)
	}
	var own, all struct{ Keys []KeyView }
	e.api("chef", http.MethodGet, "/mcp/keys", nil, &own)
	e.api("chef", http.MethodGet, "/mcp/keys?all=true", nil, &all)
	if len(own.Keys) != 1 || len(all.Keys) != 2 {
		t.Fatalf("own %d, all %d", len(own.Keys), len(all.Keys))
	}
	for _, k := range all.Keys {
		if k.ID == id && (k.Owner != "anna" || k.Own) {
			t.Fatalf("anna's key as the admin sees it: %+v", k)
		}
	}
	if code := e.api("gast", http.MethodPost, "/mcp/keys/"+id+"/revoke", nil, nil); code != http.StatusNotFound {
		t.Fatalf("another user revoking: %d", code)
	}
	if code := e.api("chef", http.MethodPost, "/mcp/keys/"+id+"/revoke", nil, nil); code != http.StatusNoContent {
		t.Fatalf("the admin revoking: %d", code)
	}
	if code, _, _ := e.raw(http.MethodPost, initBody, "Authorization", "Bearer "+key); code != http.StatusUnauthorized {
		t.Fatalf("a revoked key = %d", code)
	}
	var list struct{ Revoked []KeyView }
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if len(list.Revoked) != 1 || list.Revoked[0].RevokedReason != "admin" {
		t.Fatalf("anna sees %+v", list.Revoked)
	}
}

func TestOnlyARevokedKeyCanBeRemoved(t *testing.T) {
	e := newEnv(t)
	_, id := e.key("anna", false)
	var p problem
	if code := e.api("anna", http.MethodDelete, "/mcp/keys/"+id, nil, &p); code != http.StatusConflict || p.Code != "mcpKeyActive" {
		t.Fatalf("remove an active key: %d %s", code, p.Code)
	}
	e.api("anna", http.MethodPost, "/mcp/keys/"+id+"/revoke", nil, nil)
	if code := e.api("anna", http.MethodDelete, "/mcp/keys/"+id, nil, nil); code != http.StatusNoContent {
		t.Fatalf("remove a revoked key: %d", code)
	}
	if code := e.api("anna", http.MethodGet, "/mcp/keys/"+id+"/activity", nil, nil); code != http.StatusNotFound {
		t.Fatalf("the log of a removed key: %d", code)
	}
}

func TestAKeyNameIsShortAndPrintable(t *testing.T) {
	e := newEnv(t)
	for _, label := range []string{"", "   ", strings.Repeat("x", 65), "a‮b", "line\nbreak"} {
		var p problem
		if code := e.api("anna", http.MethodPost, "/mcp/keys", map[string]any{"label": label, "password": "richtig"}, &p); code != http.StatusBadRequest || p.Code != "mcpKeyLabel" {
			t.Fatalf("label %q: %d %s", label, code, p.Code)
		}
	}
	var p problem
	if code := e.api("anna", http.MethodPost, "/mcp/keys", map[string]any{"label": "ok", "client": "Claude Desktop", "password": "richtig"}, &p); code != http.StatusBadRequest || p.Code != "mcpKeyClient" {
		t.Fatalf("a client id with a space: %d %s", code, p.Code)
	}
}

func TestTheLogKeepsCallsButNoArguments(t *testing.T) {
	e := newEnv(t)
	key, id := e.key("anna", false)
	s := e.client(key)
	e.tool(s, "list_folder", map[string]any{"path": "/mnt/user/filme"})
	e.tool(s, "create_folder", map[string]any{"path": "/mnt/user/filme/neu"})
	var log struct{ Events []Event }
	e.api("anna", http.MethodGet, "/mcp/keys/"+id+"/activity", nil, &log)
	if len(log.Events) != 2 || log.Events[0].Tool != "create_folder" || log.Events[0].Outcome != "read_only_key" ||
		log.Events[1].Tool != "list_folder" || log.Events[1].Outcome != "ok" {
		t.Fatalf("the log = %+v", log.Events)
	}
	if raw := mustJSON(t, log); strings.Contains(raw, "filme") {
		t.Fatalf("the log keeps an argument: %s", raw)
	}
	var list struct{ Keys []KeyView }
	e.api("anna", http.MethodGet, "/mcp/keys", nil, &list)
	if list.Keys[0].CallsToday != 2 || list.Keys[0].LastUsedAt == 0 {
		t.Fatalf("the card shows %+v", list.Keys[0])
	}
}
