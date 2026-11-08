// Package api wires every feature package into /api/v1. Each package owns
// its contract and its handlers, so adding an endpoint never touches this
// file.
package api

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/filemanager"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/launch"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/office"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/remotes"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/stream"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/terminal"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/tools"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
)

// NewRouter returns the router with every package registered. The order
// here is the order of the tags in the OpenAPI document.
func NewRouter(deps v1.Deps, auth v1.Authenticator) *v1.Router {
	r := v1.NewRouter(deps, auth)
	fsapi.Routes(r)
	fileops.Routes(r)
	jobs.Routes(r)
	undo.Routes(r)
	trash.Routes(r)
	search.Routes(r)
	tools.Routes(r)
	labels.Routes(r)
	prefs.Routes(r)
	accounts.Routes(r)
	shares.Routes(r)
	unraid.Routes(r)
	remotes.Routes(r)
	system.Routes(r)
	terminal.Routes(r)
	launch.Routes(r)
	filemanager.Routes(r)
	office.Routes(r)
	stream.Routes(r)
	return r
}
