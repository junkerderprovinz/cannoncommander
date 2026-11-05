package web

import (
	"context"
	"net/http"
	"path"

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
	"github.com/junkerderprovinz/cannoncommander/backend/internal/shares"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/state"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/system"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/trash"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/unraid"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/watch"
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
func mountAPIv1(router *http.ServeMux) error {
	db := state.MainDB()
	rules, err := accounts.Rules(db)
	if err != nil {
		return err
	}
	var devices []string
	if st, err := unraid.Default().State(); err == nil {
		devices = st.Devices()
	}
	resolver, err := ccfs.FromSettings(&settings.Config, rules, devices...)
	if err != nil {
		return err
	}
	index := fsindex.Scanned()
	deps := v1.Deps{FS: resolver, Hub: eventHub, DB: db, Index: index, Users: currentUser}
	accounts.UseLogin(sessionLogin{})
	h, err := api.NewRouter(deps, accounts.Track(deps.DB, authenticateV1)).Handler()
	if err != nil {
		return err
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
	unraid.Default().SetInodeIndex(fsindex.Inodes(resolver))
	folderWatch.Start(eventHub, resolver, index, labelMover{deps})
	prefix := settings.Config.Http.BaseURL + "api/v1"
	router.Handle(prefix+"/", http.StripPrefix(prefix, h))
	mountDAV(router, deps)
	return nil
}

func currentUser(name string) (*users.User, error) {
	u, err := state.GetUserByUsername(name)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// authenticateV1 runs the same login checks as the older API, so a session
// cookie, an API token and proxy or JWT logins all work for /api/v1.
func authenticateV1(w http.ResponseWriter, r *http.Request) (*users.User, error) {
	d := &requestContext{Ctx: r.Context()}
	var user *users.User
	status, err := withUserHelper(func(_ http.ResponseWriter, _ *http.Request, d *requestContext) (int, error) {
		user = d.User
		return 0, nil
	})(w, r, d)
	if user != nil {
		return user, nil
	}
	switch status {
	case http.StatusUnauthorized, 0:
		return nil, v1.Errorf(http.StatusUnauthorized, "login required")
	case http.StatusForbidden:
		return nil, v1.Errorf(http.StatusForbidden, "access denied")
	}
	return nil, err
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
