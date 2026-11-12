// Package api wires every feature package into /api/v1. Each package owns
// its contract and its handlers, so adding an endpoint never touches this
// file.
//
// A package only the desktop app has puts "//go:build desktop" on each of
// its files and adds a container.go, built without the tag, whose Routes
// calls r.DesktopContract. The container's server then serves the same
// document and answers desktopOnly, without any of the package's code.
package api

import (
	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/clipapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/filemanager"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/launch"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/mcpapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/modules"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/office"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/openwith"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/places"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/prefs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/remotes"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/search"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shellapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/stream"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/terminal"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/tools"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/undo"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/window"
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
	openwith.Routes(r)
	shellapi.Routes(r)
	places.Routes(r)
	clipapi.Routes(r)
	launch.Routes(r)
	filemanager.Routes(r)
	window.Routes(r)
	modules.Routes(r)
	office.Routes(r)
	stream.Routes(r)
	mcpapi.Routes(r)
	return r
}
