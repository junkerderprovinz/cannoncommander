package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// viteAssetHandler serves the bundles of the Vite build. Their names carry
// a content hash, so browsers may keep them for good.
func viteAssetHandler(w http.ResponseWriter, r *http.Request) {
	name := path.Clean(strings.TrimPrefix(r.URL.Path, settings.Config.Http.BaseURL))
	name = strings.TrimPrefix(name, "/")
	if !strings.HasPrefix(name, "assets/") {
		http.NotFound(w, r)
		return
	}
	data, err := fs.ReadFile(assetFs, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	setContentType(w, name)
	_, _ = w.Write(data)
}
