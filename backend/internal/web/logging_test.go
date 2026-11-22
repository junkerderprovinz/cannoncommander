package web

import (
	"net/url"
	"strings"
	"testing"
)

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
