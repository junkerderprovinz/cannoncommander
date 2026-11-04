package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func guarded(t *testing.T, method, host string, header map[string]string) int {
	t.Helper()
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	req := httptest.NewRequest(method, "/api/v1/fs/mkdir", nil)
	req.Host = host
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "a.b.c"})
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	crossOrigin(inner).ServeHTTP(rec, req)
	return rec.Code
}

func TestAPageOnAnotherPortCannotPostWithTheSessionCookie(t *testing.T) {
	const lan = "192.168.10.10:8080"
	for name, header := range map[string]map[string]string{
		"plain http":     {"Origin": "http://192.168.10.10:9000"},
		"secure context": {"Origin": "https://192.168.10.10:9000", "Sec-Fetch-Site": "same-site"},
		"another site":   {"Origin": "https://evil.test", "Sec-Fetch-Site": "cross-site"},
		"sandboxed page": {"Origin": "null"},
	} {
		if code := guarded(t, http.MethodPost, lan, header); code != http.StatusForbidden {
			t.Errorf("%s: %d", name, code)
		}
	}
}

func TestTheAppItselfCanPost(t *testing.T) {
	for name, header := range map[string]map[string]string{
		"plain http":     {"Origin": "http://192.168.10.10:8080"},
		"secure context": {"Origin": "https://192.168.10.10:8080", "Sec-Fetch-Site": "same-origin"},
	} {
		if code := guarded(t, http.MethodPost, "192.168.10.10:8080", header); code != http.StatusNoContent {
			t.Errorf("%s: %d", name, code)
		}
	}
}

func TestAPostWithoutBrowserHeadersPasses(t *testing.T) {
	if code := guarded(t, http.MethodPost, "192.168.10.10:8080", nil); code != http.StatusNoContent {
		t.Errorf("%d", code)
	}
}

func TestReadingStaysOpenAcrossOrigins(t *testing.T) {
	header := map[string]string{"Origin": "http://192.168.10.10:9000", "Sec-Fetch-Site": "same-site"}
	if code := guarded(t, http.MethodGet, "192.168.10.10:8080", header); code != http.StatusNoContent {
		t.Errorf("%d", code)
	}
}

func TestATrustedProxyMayRewriteTheHost(t *testing.T) {
	saved := settings.Config.Http.TrustProxyHeaders
	t.Cleanup(func() { settings.Config.Http.TrustProxyHeaders = saved })
	header := map[string]string{"Origin": "http://files.home.lan", "X-Forwarded-Host": "files.home.lan"}

	settings.Config.Http.TrustProxyHeaders = false
	if code := guarded(t, http.MethodPost, "cannoncommander:8080", header); code != http.StatusForbidden {
		t.Errorf("a forwarded host counted without trust: %d", code)
	}
	settings.Config.Http.TrustProxyHeaders = true
	if code := guarded(t, http.MethodPost, "cannoncommander:8080", header); code != http.StatusNoContent {
		t.Errorf("behind the proxy: %d", code)
	}
	header["Origin"] = "http://evil.test"
	if code := guarded(t, http.MethodPost, "cannoncommander:8080", header); code != http.StatusForbidden {
		t.Errorf("another site behind the proxy: %d", code)
	}
}

func TestTheExternalAddressCountsAsTheApp(t *testing.T) {
	saved := settings.Config.Http.ExternalUrl
	t.Cleanup(func() { settings.Config.Http.ExternalUrl = saved })
	settings.Config.Http.ExternalUrl = "https://files.home.lan/cannon/"
	header := map[string]string{"Origin": "https://files.home.lan"}
	if code := guarded(t, http.MethodPost, "cannoncommander:8080", header); code != http.StatusNoContent {
		t.Errorf("%d", code)
	}
}

func headersFrom(t *testing.T, inner http.HandlerFunc) http.Header {
	t.Helper()
	rec := httptest.NewRecorder()
	securityHeaders(inner).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	return rec.Header()
}

func TestEveryResponseCarriesTheSecurityHeaders(t *testing.T) {
	h := headersFrom(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	if h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "same-origin" {
		t.Errorf("headers %v", h)
	}
	if csp := h.Get("Content-Security-Policy"); csp != "frame-ancestors 'self'" {
		t.Errorf("policy %q", csp)
	}
	if h.Get("Strict-Transport-Security") != "" {
		t.Error("HSTS belongs to the proxy")
	}
}

func TestAHandlersOwnPolicyStays(t *testing.T) {
	h := headersFrom(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Security-Policy", fsapi.SandboxPolicy)
		w.Header().Set("Referrer-Policy", "no-referrer")
	})
	if h.Get("Content-Security-Policy") != fsapi.SandboxPolicy || h.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("headers %v", h)
	}
}

func TestEveryPolicyKeepsOtherSitesFromFramingTheApp(t *testing.T) {
	for name, policy := range map[string]string{"sandbox": fsapi.SandboxPolicy, "app": appPolicy(), "share": sharePagePolicy} {
		if !strings.Contains(policy, "frame-ancestors 'self'") {
			t.Errorf("%s policy %q", name, policy)
		}
	}
}

func TestTheAppPolicyClosesBaseAndPlugins(t *testing.T) {
	policy := appPolicy()
	for _, want := range []string{"default-src 'self'", "base-uri 'self'", "object-src 'none'"} {
		if !strings.Contains(policy, want) {
			t.Errorf("policy %q lacks %s", policy, want)
		}
	}
}
