package system

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// fakeOffice is a document server that wants tokens signed with secret,
// or none when secret is empty.
func fakeOffice(t *testing.T, secret string) *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthcheck", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("true")) })
	mux.HandleFunc("POST /coauthoring/CommandService.ashx", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["c"] != "version" {
			t.Errorf("command %v", body["c"])
		}
		if secret != "" {
			raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			tok, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return []byte(secret), nil })
			if err != nil || !tok.Valid {
				_, _ = w.Write([]byte(`{"error":6}`))
				return
			}
		}
		_, _ = w.Write([]byte(`{"error":0,"version":"8.2.1.38"}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestOfficeTestChecksTheJWTSecret(t *testing.T) {
	locked := fakeOffice(t, "geheim")
	open := fakeOffice(t, "")
	tr := newTester()
	ctx := context.Background()
	for _, tc := range []struct {
		url, secret, token string
		ok                 bool
	}{
		{locked.URL, "geheim", TokenOK, true},
		{locked.URL, "falsch", TokenRejected, false},
		{locked.URL, "", TokenMissing, false},
		{open.URL, "", TokenOff, true},
	} {
		res := tr.run(ctx, KindOnlyOffice, tc.url, tc.secret)
		if !res.Reachable || res.Token != tc.token || res.OK != tc.ok {
			t.Errorf("secret %q: %+v", tc.secret, res)
		}
		if tc.ok && res.Version != "8.2.1.38" {
			t.Errorf("version %q", res.Version)
		}
	}
	res := tr.run(ctx, KindEuroOffice, "http://127.0.0.1:1", "")
	if res.Reachable || res.OK || res.Error == "" || res.Kind != KindEuroOffice {
		t.Errorf("nothing listening: %+v", res)
	}
}

func TestOfficeTestFallsBackToTheNewCommandPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthcheck", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("true")) })
	mux.HandleFunc("POST /command", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":0,"version":"9.0.4"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	if res := newTester().run(context.Background(), KindOnlyOffice, srv.URL, ""); !res.OK || res.Version != "9.0.4" {
		t.Fatalf("%+v", res)
	}
}

func TestOfficeTestTellsAWebsiteFromADocumentServer(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	if res := newTester().run(context.Background(), KindOnlyOffice, srv.URL, ""); res.Reachable || res.OK || res.Error == "" {
		t.Fatalf("%+v", res)
	}
}

func TestCollaboraTestReadsTheDiscovery(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hosting/discovery", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<?xml version="1.0"?><wopi-discovery><net-zone/></wopi-discovery>`))
	})
	mux.HandleFunc("GET /hosting/capabilities", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"productVersion":"24.04.9.2"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	res := newTester().run(context.Background(), KindCollabora, srv.URL, "")
	if !res.OK || res.Token != TokenNone || res.Version != "24.04.9.2" {
		t.Fatalf("%+v", res)
	}
}

func TestArrowLoopTestChecksTheToken(t *testing.T) {
	fake := func(required bool) *httptest.Server {
		mux := http.NewServeMux()
		mux.HandleFunc("GET /api/session", func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]bool{"required": required, "authenticated": !required})
		})
		mux.HandleFunc("GET /api/jobs", func(w http.ResponseWriter, r *http.Request) {
			if required && r.Header.Get("X-ArrowLoop-Token") != "richtig" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(`[]`))
		})
		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)
		return srv
	}
	locked, open := fake(true), fake(false)
	tr := newTester()
	ctx := context.Background()
	for _, tc := range []struct {
		url, token, want string
	}{
		{locked.URL, "richtig", TokenOK},
		{locked.URL, "falsch", TokenRejected},
		{locked.URL, "", TokenMissing},
		{open.URL, "", TokenOff},
	} {
		if res := tr.run(ctx, KindArrowLoop, tc.url, tc.token); !res.Reachable || res.Token != tc.want {
			t.Errorf("token %q: %+v", tc.token, res)
		}
	}
}
