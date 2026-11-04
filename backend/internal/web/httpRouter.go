package web

import (
	"net/http"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// configureHTTPRouter registers what lives outside /api/v1: the pages, the
// bundles, the single sign on that the sign in page links to, and WebDAV.
func configureHTTPRouter(router *http.ServeMux) {
	base := settings.Config.Http.BaseURL

	oidc := http.NewServeMux()
	oidc.HandleFunc("GET /auth/oidc/login", withRateLimit(AuthRateLimitOIDC, oidcLoginHandler))
	oidc.HandleFunc("GET /auth/oidc/callback", withRateLimit(AuthRateLimitOIDC, oidcCallbackHandler))
	router.Handle(base+"api/auth/oidc/", http.StripPrefix(base+"api", oidc))
	// Anything else under /api is a mistyped call, not a page of the app.
	router.Handle(base+"api/", http.NotFoundHandler())

	if !settings.Config.Http.DisableWebDAV {
		// The webdav library needs the /dav prefix, so it stays on the path.
		router.Handle(base+"dav/{source}/{path...}", withBasicAuth(webDAVHandler))
	}

	router.HandleFunc("GET "+base+"assets/", viteAssetHandler)
	router.HandleFunc("GET "+base+"s/{slug}", sharePageHandler)
	router.HandleFunc("GET "+base+"health", healthHandler)
	router.HandleFunc(base, indexHandler)

	if base != "/" {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, base, http.StatusMovedPermanently)
		})
	}
}
