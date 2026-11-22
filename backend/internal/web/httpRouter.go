package web

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// configureHTTPRouter registers what lives outside /api/v1 and the sign-in:
// the pages and the bundles.
func configureHTTPRouter(router *http.ServeMux) {
	base := settings.Config.Http.BaseURL

	// Anything else under /api is a mistyped call, not a page of the app.
	router.Handle(base+"api/", http.NotFoundHandler())

	router.HandleFunc("GET "+base+"assets/", viteAssetHandler)
	for _, name := range []string{"manifest.webmanifest", "manifest/", "sw.js", "icons/"} {
		router.HandleFunc("GET "+base+name, rootFileHandler)
	}
	router.HandleFunc("POST "+base+"teilen", sharedTooEarly)
	router.HandleFunc("GET "+base+"s/{slug}", sharePageHandler)
	router.HandleFunc("GET "+base+"health", healthHandler)
	router.HandleFunc(base, indexHandler)

	if base != "/" {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, base, http.StatusMovedPermanently)
		})
	}
}
