package web

import "testing"

func TestSafeRelativeRedirectPath(t *testing.T) {
	tests := []struct {
		raw  string
		want string
		ok   bool
	}{
		{"/files/", "/files/", true},
		{"/files/?q=1", "/files/?q=1", true},
		{"", "", false},
		{"files/", "", false},
		{"//evil.example/", "", false},
		{"/\\evil.example/", "", false},
		{"https://evil.example/", "", false},
		{"/files/\n", "", false},
		{"%2Ffiles%2F", "/files/", true},
		{"/files/?q=a+b", "/files/?q=a+b", true},
	}
	for _, tt := range tests {
		got, ok := SafeRelativeRedirectPath(tt.raw)
		if ok != tt.ok || got != tt.want {
			t.Errorf("SafeRelativeRedirectPath(%q) = (%q, %v), want (%q, %v)", tt.raw, got, ok, tt.want, tt.ok)
		}
	}
}

func TestSafeRelativeRedirectPathRefusesWhatBrowsersReadAsAnotherHost(t *testing.T) {
	for _, raw := range []string{
		"/%09/evil.test",
		"/%2509/evil.test",
		"/%252509/evil.test",
		"/%25252509/evil.test",
		"/%2525252525252525252509/evil.test",
		"/\t/evil.test",
		"/%0a/evil.test",
		"/%250d/evil.test",
		"/%7F/evil.test",
		"/%5C/evil.test",
		"/%255Cevil.test",
		`/files\evil.test`,
		"/%2F/evil.test",
		"/%252F/evil.test",
		"%2F%2Fevil.test",
		"%252F%252Fevil.test",
		"/x%3A%2F%2Fevil.test",
	} {
		if got, ok := SafeRelativeRedirectPath(raw); ok {
			t.Errorf("SafeRelativeRedirectPath(%q) = %q", raw, got)
		}
	}
}

// The value travels through a cookie and is checked again at each step, so
// a later check must see what the first one approved.
func TestSafeRelativeRedirectPathReturnsAValueThatStaysTheSame(t *testing.T) {
	for _, raw := range []string{"/files/", "/files/?q=a+b", "%2Ffiles%2F", "/files/a%20b", "/files/100%25.txt", "/%252Ffiles"} {
		got, ok := SafeRelativeRedirectPath(raw)
		if !ok {
			continue
		}
		again, ok := SafeRelativeRedirectPath(got)
		if !ok || again != got {
			t.Errorf("%q gave %q, then %q %v", raw, got, again, ok)
		}
	}
}
