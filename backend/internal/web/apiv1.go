package web

import (
	"context"
	"net/http"
	"path"
	"path/filepath"

	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/accounts"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/api"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fileops"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/fsindex"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/hub"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/jobs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/labels"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/mcpapi"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/places"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/terminal"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/settings"
)

// The watcher follows the hub's folder topics, and both live as long as
// the process.
var (
	folderWatch = watch.New(watch.Options{})
	eventHub    = hub.New(hub.Options{OnTopic: folderWatch.Topic})
)

// mountAPIv1 serves /api/v1. The more specific prefix wins over /api/ in
// the ServeMux.
// It returns what finds the visitor of each request, which goes in front of
// the whole router.
func mountAPIv1(ctx context.Context, router *http.ServeMux) (func(http.Handler) http.Handler, error) {
	db := state.MainDB()
	rules, err := accounts.Rules(db)
	if err != nil {
		return nil, err
	}
	resolver, err := ccfs.FromSettings(&settings.Config, rules)
	if err != nil {
		return nil, err
	}
	svc := unraid.Default()
	var devices []string
	if st, err := svc.State(); err == nil {
		devices = st.Devices()
	}
	resolver = resolver.WithDevices(filepath.Dir(svc.UserShares()), devices...)
	resolver = resolver.WithExclusive(svc)
	if dbPath := settings.Config.Server.DatabaseV2.Path; dbPath != "" {
		resolver = resolver.WithDataFolder(filepath.Dir(dbPath))
	}
	resolver = places.Follow(resolver, eventHub)
	index := fsindex.Scanned()
	deps := v1.Deps{FS: resolver, Hub: eventHub, DB: db, Index: index, Users: currentUser}
	guard, err := mountSignIn(ctx, router, deps)
	if err != nil {
		return nil, err
	}
	system.OnShutdown(accounts.Close)
	accounts.UseClientIP(GetRemoteIP)
	terminal.UseGate(terminalGate{guard: originGuard()})
	h, err := api.NewRouter(deps, mcpapi.Authenticate(accounts.Authenticate)).Handler()
	if err != nil {
		return nil, err
	}
	// Hooks run last added first, so running jobs stop before the trash and
	// the share links they may still write to.
	if s := shares.For(deps); s != nil {
		system.OnShutdown(s.Close)
	}
	if s := trash.For(deps); s != nil {
		system.OnShutdown(s.Close)
	}
	if m := jobs.For(deps); m != nil {
		system.OnShutdown(m.Close)
	}
	followDevices(svc, resolver, eventHub, devices)
	svc.SetInodeIndex(fsindex.Inodes(resolver))
	folderWatch.Start(eventHub, resolver, index, labelMover{deps})
	desktop.Listen(func(typ string, data any) { _, _ = eventHub.PublishJSON(typ, "", data) })
	system.OnShutdown(fsindex.Follow(resolver, index))
	prefix := settings.Config.Http.BaseURL + "api/v1"
	router.Handle(prefix+"/", http.StripPrefix(prefix, h))
	if s := mcpapi.For(deps); s != nil {
		s.Mount(router, settings.Config.Http.BaseURL, h)
	}
	mountDAV(router, deps)
	return guard, nil
}

func currentUser(name string) (*users.User, error) {
	u, err := state.GetUserByUsername(name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// labelMover lets labels follow files that something outside the app
// renamed, such as a download client finishing a part file.
type labelMover struct{ deps v1.Deps }

func (m labelMover) Moved(from, to string) {
	// A finished part file takes the place of the file it replaces, whose
	// labels stay.
	if fileops.IsPart(path.Base(from)) {
		return
	}
	if err := labels.Moved(context.Background(), m.deps, labels.Move{From: from, To: to}); err != nil {
		logger.Errorf("labels for %s moved to %s: %v", from, to, err)
	}
}
