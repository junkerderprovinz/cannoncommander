package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

func TestTheSessionCookieIsSecureOnTheHttpsAddressBehindAPlainProxy(t *testing.T) {
	orig := settings.Config.Http.ExternalUrl
	t.Cleanup(func() { settings.Config.Http.ExternalUrl = orig })
	settings.Config.Http.ExternalUrl = "https://files.example.com/"

	cookie := func(host string) *http.Cookie {
		req, err := http.NewRequest(http.MethodGet, "http://"+host+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		return sessionCookie(req, "token", time.Now().Add(time.Hour), 0, http.SameSiteStrictMode)
	}
	if !cookie("files.example.com").Secure {
		t.Error("the external address gets a cookie without Secure")
	}
	if cookie("192.168.10.10:8080").Secure {
		t.Error("a browser would drop a Secure cookie on the plain LAN address")
	}
	settings.Config.Http.ExternalUrl = "http://files.example.com/"
	if cookie("files.example.com").Secure {
		t.Error("a plain external address cannot keep a Secure cookie")
	}
}

func TestLoggedAddressesLeaveOutWhatGrantsAccess(t *testing.T) {
	cases := []struct{ raw, want string }{
		{"/api/v1/office/file?token=geheim", "/api/v1/office/file?token=***"},
		{"/api/v1/office/wopi/files/1?access_token=geheim&x=1", "/api/v1/office/wopi/files/1?access_token=***&x=1"},
		{"/api/auth/oidc/callback?code=geheim&state=auch", "/api/auth/oidc/callback?code=***&state=***"},
		{"/api/v1/fs/list?path=/mnt/user&sort=name", "/api/v1/fs/list?path=/mnt/user&sort=name"},
		{"/api/v1/fs/list", "/api/v1/fs/list"},
		{"/x?TOKEN=geheim", "/x?TOKEN=***"},
	}
	for _, c := range cases {
		u, err := url.Parse(c.raw)
		if err != nil {
			t.Fatal(err)
		}
		got := logSafeURL(u)
		if got != c.want {
			t.Errorf("%s: %s", c.raw, got)
		}
		if strings.Contains(got, "geheim") {
			t.Errorf("%s leaks", c.raw)
		}
	}
}

func TestAnUnverifiedEmailDoesNotNameAnAccount(t *testing.T) {
	cases := []struct {
		name       string
		identifier string
		claims     map[string]any
		want       string
		refused    bool
	}{
		{"a name", "preferred_username", map[string]any{"preferred_username": "anna", "email_verified": false}, "anna", false},
		{"a verified email", "email", map[string]any{"email": "anna@example.com", "email_verified": true}, "anna@example.com", false},
		{"no word on it", "email", map[string]any{"email": "anna@example.com"}, "anna@example.com", false},
		{"an unverified email", "email", map[string]any{"email": "anna@example.com", "email_verified": false}, "", true},
		{"no such claim", "email", map[string]any{"preferred_username": "anna"}, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := oidcLoginName(c.claims, c.identifier)
			if (err != nil) != c.refused || got != c.want {
				t.Errorf("%q, %v", got, err)
			}
		})
	}
}
