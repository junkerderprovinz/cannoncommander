//go:build !desktop

package places

import (
	_ "embed"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
)

//go:embed openapi.yaml
var contract []byte

// Routes serves the contract without the feature, which only the desktop app
// has.
func Routes(r *v1.Router) { r.DesktopContract("places", contract) }

// Follow returns fs as it is: the container reaches its configured sources
// and nothing else.
func Follow(fs *ccfs.Resolver, _ *hub.Hub) *ccfs.Resolver { return fs }
