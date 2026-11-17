package storeroom

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
)

func (b *box) handler() http.Handler {
	b.t.Helper()
	people := map[string]*users.User{"jdp": {Username: "jdp"}, "anna": {Username: "anna"}}
	people["jdp"].Permissions.Admin = true
	auth := func(_ http.ResponseWriter, r *http.Request) (*users.User, error) {
		if u, ok := people[r.Header.Get("X-User")]; ok {
			return u, nil
		}
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	}
	router := v1.NewRouter(v1.Deps{}, auth)
	router.Contract("storeroom", contract)
	bind(router, b.svc)
	h, err := router.Handler()
	if err != nil {
		b.t.Fatal(err)
	}
	return h
}

func send(h http.Handler, who, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set("X-User", who)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestOnlyAnAdminSeesTheStoreroom(t *testing.T) {
	b := newBox(t)
	b.template("alt", "")
	h := b.handler()
	if rec := send(h, "anna", http.MethodGet, "/storeroom/scan", ""); rec.Code != http.StatusForbidden {
		t.Errorf("anna gets %d", rec.Code)
	}
	rec := send(h, "jdp", http.MethodGet, "/storeroom/scan", "")
	var s Scan
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil || rec.Code != http.StatusOK || len(s.Finds) != 1 {
		t.Errorf("jdp gets %d %s", rec.Code, rec.Body)
	}
}

func TestThePlanAnswersOverHTTPAndRunNeedsWriteAccess(t *testing.T) {
	b := newBox(t)
	tpl := b.template("alt", "")
	b.svc.cfg.Write = false
	h := b.handler()
	body := `{"op":"store","refs":[{"kind":"template","id":` + jsonString(tpl) + `}]}`
	rec := send(h, "jdp", http.MethodPost, "/storeroom/plan", body)
	var out struct{ Steps []Step }
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Steps) != 1 || out.Steps[0].Problem != "" {
		t.Fatalf("plan: %d %s", rec.Code, rec.Body)
	}
	rec = send(h, "jdp", http.MethodPost, "/storeroom/run", body)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "unraidReadOnly") || !exists(tpl) {
		t.Errorf("run without write access: %d %s", rec.Code, rec.Body)
	}
	if rec := send(h, "jdp", http.MethodPost, "/storeroom/plan", `{"op":"burn","refs":[]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown op: %d", rec.Code)
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
