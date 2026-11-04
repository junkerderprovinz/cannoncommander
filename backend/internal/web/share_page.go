package web

import (
	"io/fs"
	"net/http"
)

// sharePagePolicy keeps the page for visitors to its own scripts. Styles may
// be inline because Vue sets them on elements.
const sharePagePolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"font-src 'self' https://fonts.gstatic.com; img-src 'self' blob: data:; media-src 'self' blob:; frame-src 'self'; " +
	"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'self'"

// sharePageHandler serves the page a share link opens. It is an entry of
// its own in the build, so a visitor never loads the file manager.
func sharePageHandler(w http.ResponseWriter, r *http.Request) {
	page, err := fs.ReadFile(assetFs, "share.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	h.Set("Content-Security-Policy", sharePagePolicy)
	// The address is the key to the files, so it must not travel to other sites.
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("X-Robots-Tag", "noindex, nofollow")
	_, _ = w.Write(page)
}
