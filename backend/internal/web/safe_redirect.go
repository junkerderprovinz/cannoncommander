package web

import (
	"net/url"
	"strings"
	"unicode"
)

// maxRedirectEncodings is deeper than any link the app writes, so a value
// still changing after that many rounds is an attempt to outlast the check.
const maxRedirectEncodings = 4

// SafeRelativeRedirectPath reports whether raw is a safe in-app path for
// post-login redirects and returns it fully decoded. The value comes back here
// from a cookie, so each check has to see what the browser will finally get:
// browsers drop tabs and newlines and read a backslash as a slash, which turns
// "/<TAB>/evil.test" into "//evil.test".
func SafeRelativeRedirectPath(raw string) (string, bool) {
	decoded := raw
	for round := 0; ; round++ {
		next, err := url.PathUnescape(decoded)
		if err != nil || next == decoded {
			break
		}
		if round == maxRedirectEncodings {
			return "", false
		}
		decoded = next
	}
	if strings.ContainsFunc(decoded, unicode.IsControl) || strings.Contains(decoded, `\`) {
		return "", false
	}
	decoded = strings.TrimSpace(decoded)
	if !strings.HasPrefix(decoded, "/") || strings.HasPrefix(decoded, "//") || strings.Contains(decoded, "://") {
		return "", false
	}
	return decoded, true
}
