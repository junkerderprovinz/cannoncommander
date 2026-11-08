package launch

import (
	"net/url"
	"slices"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/shortcut"
)

// toOpen gives the address of an internet shortcut, so a double click opens
// the page in the browser. The Linux desktops know no program for a .url,
// and Windows would first warn about the file itself when it lies on a share.
// Only addresses of the web and of mail are handed on.
func toOpen(file string) string {
	if shortcut.KindOf(file) != shortcut.URL {
		return file
	}
	sc, err := shortcut.Read(file)
	if err != nil {
		return file
	}
	if u, err := url.Parse(sc.Target); err == nil && slices.Contains([]string{"http", "https", "ftp", "mailto"}, strings.ToLower(u.Scheme)) {
		return sc.Target
	}
	return file
}
