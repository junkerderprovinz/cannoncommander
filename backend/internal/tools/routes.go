// Package tools holds the tools of the Werkzeuge menu: archives,
// checksums, splitting files, links and rights, renaming by pattern,
// comparing and syncing folders, comparing texts, cleaning up, loading
// from an address and handing a sync to ArrowLoop.
package tools

import (
	"context"
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
)

//go:embed openapi.yaml
var contract []byte

type handlers struct {
	deps            v1.Deps
	fs              *ccfs.Resolver
	meta            Meta
	arrowLoopConfig func(context.Context) (ArrowLoopConfig, error)
}

// Routes registers the contract and the handlers. The batch rename, the
// links and the rights belong to the fileops contract, which has to be
// registered first.
func Routes(r *v1.Router) {
	r.Contract("tools", contract)
	d := r.Deps()
	if d.FS == nil {
		return
	}
	h := handlers{deps: d, fs: d.FS, meta: readMeta}
	h.arrowLoopConfig = h.savedArrowLoop
	bind(r, h)
}

func bind(r *v1.Router, h handlers) {
	r.Handle("POST /fs/rename/batch", h.renameBatch)
	r.Handle("POST /fs/link", h.link)
	r.Handle("PATCH /fs/attrs", h.setAttrs)
	r.Handle("POST /tools/compare", h.compare)
	r.Handle("POST /tools/sync/plan", h.syncPlan)
	r.Handle("POST /tools/diff", h.diff)
	r.Handle("POST /tools/cleanup/scan", h.cleanupScan)
	r.Handle("POST /tools/checksum/parse", h.checksumParse)
	r.Handle("GET /tools/archives", h.formats)
	r.Handle("GET /tools/archive", h.archiveInfo)
	r.Handle("POST /tools/arrowloop", h.arrowLoop)
}
