package web

import (
	"io/fs"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/adapters/fs/fileutils"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// indexTemplatePath finds index.html in the asset files. The Vite build of
// the new interface puts it at the top, the older build under public/.
func indexTemplatePath() string {
	if settings.Env.IsDevMode {
		return "index.html"
	}
	if assets := fileutils.GetAssetFS(); assets != nil {
		if _, err := fs.Stat(assets, "index.html"); err == nil {
			return "index.html"
		}
	}
	return "public/index.html"
}
